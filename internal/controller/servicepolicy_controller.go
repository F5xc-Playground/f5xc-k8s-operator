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

type ServicePolicyReconciler struct {
	client.Client
	Log       logr.Logger
	ClientSet *xcclientset.ClientSet
}

// +kubebuilder:rbac:groups=xc.f5.com,resources=servicepolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=xc.f5.com,resources=servicepolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=xc.f5.com,resources=servicepolicies/finalizers,verbs=update

func (r *ServicePolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("servicepolicy", req.NamespacedName)

	var cr v1alpha1.ServicePolicy
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

	xcNS := resolveServicePolicyXCNamespace(&cr)
	xc := r.ClientSet.Get()

	current, err := xc.GetServicePolicy(ctx, xcNS, cr.Name)
	if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
		return r.handleXCError(ctx, log, &cr, err, "get")
	}

	if errors.Is(err, xcclient.ErrNotFound) {
		return r.handleCreate(ctx, log, &cr, xc, xcNS)
	}

	return r.handleUpdate(ctx, log, &cr, xc, xcNS, current)
}

func (r *ServicePolicyReconciler) handleCreate(ctx context.Context, log logr.Logger, cr *v1alpha1.ServicePolicy, xc xcclient.XCClient, xcNS string) (ctrl.Result, error) {
	create := buildServicePolicyCreate(cr, xcNS)
	result, err := xc.CreateServicePolicy(ctx, xcNS, create)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "create")
	}

	log.Info("created XC service policy", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonCreateSucceeded, "XC service policy created", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ServicePolicyReconciler) handleUpdate(ctx context.Context, log logr.Logger, cr *v1alpha1.ServicePolicy, xc xcclient.XCClient, xcNS string, current *xcclient.ServicePolicy) (ctrl.Result, error) {
	desiredJSON, err := buildServicePolicyDesiredSpecJSON(cr, xcNS)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building desired spec JSON: %w", err)
	}

	needsUpdate, err := xc.ClientNeedsUpdate(current.RawSpec, desiredJSON)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("comparing specs: %w", err)
	}

	if !needsUpdate {
		r.setStatus(cr, true, true, v1alpha1.ReasonUpToDate, "XC service policy is up to date", current)
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	replace := buildServicePolicyReplace(cr, xcNS, current.Metadata.ResourceVersion)
	result, err := xc.ReplaceServicePolicy(ctx, xcNS, cr.Name, replace)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "update")
	}

	log.Info("updated XC service policy", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonUpdateSucceeded, "XC service policy updated", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ServicePolicyReconciler) handleDeletion(ctx context.Context, log logr.Logger, cr *v1alpha1.ServicePolicy) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, v1alpha1.FinalizerXCCleanup) {
		return ctrl.Result{}, nil
	}

	policy := cr.Annotations[v1alpha1.AnnotationDeletionPolicy]
	if policy != v1alpha1.DeletionPolicyOrphan {
		xcNS := resolveServicePolicyXCNamespace(cr)
		xc := r.ClientSet.Get()
		err := xc.DeleteServicePolicy(ctx, xcNS, cr.Name)
		if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
			return r.handleXCError(ctx, log, cr, err, "delete")
		}
		log.Info("deleted XC service policy", "name", cr.Name, "xcNamespace", xcNS)
	} else {
		log.Info("orphaning XC service policy", "name", cr.Name)
	}

	controllerutil.RemoveFinalizer(cr, v1alpha1.FinalizerXCCleanup)
	if err := r.Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ServicePolicyReconciler) handleXCError(ctx context.Context, log logr.Logger, cr *v1alpha1.ServicePolicy, err error, operation string) (ctrl.Result, error) {
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
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonConflict, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{Requeue: true}, nil

	case errors.Is(err, xcclient.ErrRateLimited):
		log.Info("rate limited by XC API, requeueing", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonRateLimited, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil

	case errors.Is(err, xcclient.ErrServerError):
		log.Error(err, "XC API server error, requeueing", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonServerError, err.Error())
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

func (r *ServicePolicyReconciler) setStatus(cr *v1alpha1.ServicePolicy, ready, synced bool, reason, message string, xcObj *xcclient.ServicePolicy) {
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

func (r *ServicePolicyReconciler) setCondition(cr *v1alpha1.ServicePolicy, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: cr.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func resolveServicePolicyXCNamespace(cr *v1alpha1.ServicePolicy) string {
	if ns, ok := cr.Annotations[v1alpha1.AnnotationXCNamespace]; ok && ns != "" {
		return ns
	}
	return cr.Namespace
}

func (r *ServicePolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ServicePolicy{}).
		Complete(r)
}
