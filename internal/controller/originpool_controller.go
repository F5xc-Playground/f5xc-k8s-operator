package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

type OriginPoolReconciler struct {
	client.Client
	Log       logr.Logger
	ClientSet *xcclientset.ClientSet
}

// +kubebuilder:rbac:groups=xc.f5.com,resources=originpools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=xc.f5.com,resources=originpools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=xc.f5.com,resources=originpools/finalizers,verbs=update

func (r *OriginPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("originpool", req.NamespacedName)

	var cr v1alpha1.OriginPool
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !cr.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, log, &cr)
	}

	if !controllerutil.ContainsFinalizer(&cr, v1alpha1.FinalizerXCCleanup) {
		controllerutil.AddFinalizer(&cr, v1alpha1.FinalizerXCCleanup)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	xcNS := resolveXCNamespace(&cr)
	xc := r.ClientSet.Get()

	current, err := xc.GetOriginPool(ctx, xcNS, cr.Name)
	if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
		return r.handleXCError(ctx, log, &cr, err, "get")
	}

	if errors.Is(err, xcclient.ErrNotFound) {
		return r.handleCreate(ctx, log, &cr, xc, xcNS)
	}

	return r.handleUpdate(ctx, log, &cr, xc, xcNS, current)
}

func (r *OriginPoolReconciler) handleCreate(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool, xc xcclient.XCClient, xcNS string) (ctrl.Result, error) {
	create := buildOriginPoolCreate(cr, xcNS)
	result, err := xc.CreateOriginPool(ctx, xcNS, create)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "create")
	}

	log.Info("created XC origin pool", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonCreateSucceeded, "XC origin pool created", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *OriginPoolReconciler) handleUpdate(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool, xc xcclient.XCClient, xcNS string, current *xcclient.OriginPool) (ctrl.Result, error) {
	desiredJSON, err := buildOriginPoolDesiredSpecJSON(cr, xcNS)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building desired spec JSON: %w", err)
	}

	needsUpdate, err := xc.ClientNeedsUpdate(current.RawSpec, desiredJSON)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("comparing specs: %w", err)
	}

	if !needsUpdate {
		r.setStatus(cr, true, true, v1alpha1.ReasonUpToDate, "XC origin pool is up to date", current)
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	replace := buildOriginPoolReplace(cr, xcNS, current.Metadata.ResourceVersion)
	result, err := xc.ReplaceOriginPool(ctx, xcNS, cr.Name, replace)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "update")
	}

	log.Info("updated XC origin pool", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonUpdateSucceeded, "XC origin pool updated", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *OriginPoolReconciler) handleDeletion(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, v1alpha1.FinalizerXCCleanup) {
		return ctrl.Result{}, nil
	}

	policy := cr.Annotations[v1alpha1.AnnotationDeletionPolicy]
	if policy != v1alpha1.DeletionPolicyOrphan {
		xcNS := resolveXCNamespace(cr)
		xc := r.ClientSet.Get()
		err := xc.DeleteOriginPool(ctx, xcNS, cr.Name)
		if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
			return r.handleXCError(ctx, log, cr, err, "delete")
		}
		log.Info("deleted XC origin pool", "name", cr.Name, "xcNamespace", xcNS)
	} else {
		log.Info("orphaning XC origin pool", "name", cr.Name)
	}

	controllerutil.RemoveFinalizer(cr, v1alpha1.FinalizerXCCleanup)
	if err := r.Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *OriginPoolReconciler) handleXCError(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool, err error, operation string) (ctrl.Result, error) {
	switch {
	case errors.Is(err, xcclient.ErrAuth):
		log.Error(err, "authentication failure — not retrying", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonAuthFailure, err.Error())
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonAuthFailure, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{}, nil

	case errors.Is(err, xcclient.ErrConflict):
		log.Info("conflict on XC API, requeueing immediately", "operation", operation)
		reason := v1alpha1.ReasonConflict
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, reason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{Requeue: true}, nil

	case errors.Is(err, xcclient.ErrRateLimited):
		log.Info("rate limited by XC API, requeueing", "operation", operation)
		reason := v1alpha1.ReasonRateLimited
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, reason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil

	case errors.Is(err, xcclient.ErrServerError):
		log.Error(err, "XC API server error, requeueing", "operation", operation)
		reason := v1alpha1.ReasonServerError
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, reason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	default:
		log.Error(err, "XC API error, requeueing", "operation", operation)
		failReason := operationFailReason(operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, failReason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

func (r *OriginPoolReconciler) setStatus(cr *v1alpha1.OriginPool, ready, synced bool, reason, message string, xcObj *xcclient.OriginPool) {
	readyStatus := metav1.ConditionTrue
	if !ready {
		readyStatus = metav1.ConditionFalse
	}
	syncedStatus := metav1.ConditionTrue
	if !synced {
		syncedStatus = metav1.ConditionFalse
	}

	r.setCondition(cr, v1alpha1.ConditionReady, readyStatus, reason, message)
	r.setCondition(cr, v1alpha1.ConditionSynced, syncedStatus, reason, message)
	cr.Status.ObservedGeneration = cr.Generation

	if xcObj != nil {
		cr.Status.XCResourceVersion = xcObj.Metadata.ResourceVersion
		cr.Status.XCUID = xcObj.SystemMetadata.UID
		cr.Status.XCNamespace = xcObj.Metadata.Namespace
	}
}

func (r *OriginPoolReconciler) setCondition(cr *v1alpha1.OriginPool, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: cr.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func resolveXCNamespace(cr *v1alpha1.OriginPool) string {
	if ns, ok := cr.Annotations[v1alpha1.AnnotationXCNamespace]; ok && ns != "" {
		return ns
	}
	return cr.Namespace
}

func operationFailReason(op string) string {
	switch op {
	case "create":
		return v1alpha1.ReasonCreateFailed
	case "update":
		return v1alpha1.ReasonUpdateFailed
	case "delete":
		return v1alpha1.ReasonDeleteFailed
	default:
		return v1alpha1.ReasonCreateFailed
	}
}

func (r *OriginPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.OriginPool{}).
		Complete(r)
}
