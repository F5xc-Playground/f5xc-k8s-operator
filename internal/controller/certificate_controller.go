package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

type CertificateReconciler struct {
	client.Client
	Log       logr.Logger
	ClientSet *xcclientset.ClientSet
}

// +kubebuilder:rbac:groups=xc.f5.com,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=xc.f5.com,resources=certificates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=xc.f5.com,resources=certificates/finalizers,verbs=update

func (r *CertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("certificate", req.NamespacedName)

	var cr v1alpha1.Certificate
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

	certPEM, keyPEM, err := r.readTLSSecret(ctx, &cr)
	if err != nil {
		log.Error(err, "failed to read TLS secret")
		r.setCondition(&cr, v1alpha1.ConditionReady, metav1.ConditionFalse, "SecretReadFailed", err.Error())
		r.setCondition(&cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, "SecretReadFailed", err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, &cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	xcNS := cr.Spec.XCNamespace
	xc := r.ClientSet.Get()

	current, err := xc.GetCertificate(ctx, xcNS, cr.Name)
	if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
		return r.handleXCError(ctx, log, &cr, err, "get")
	}

	if errors.Is(err, xcclient.ErrNotFound) {
		return r.handleCreate(ctx, log, &cr, xc, xcNS, certPEM, keyPEM)
	}

	return r.handleUpdate(ctx, log, &cr, xc, xcNS, current, certPEM, keyPEM)
}

func (r *CertificateReconciler) readTLSSecret(ctx context.Context, cr *v1alpha1.Certificate) ([]byte, []byte, error) {
	secretNS := cr.Spec.SecretRef.Namespace
	if secretNS == "" {
		secretNS = cr.Namespace
	}

	var secret corev1.Secret
	key := types.NamespacedName{Namespace: secretNS, Name: cr.Spec.SecretRef.Name}
	if err := r.Get(ctx, key, &secret); err != nil {
		return nil, nil, fmt.Errorf("getting Secret %s: %w", key, err)
	}

	if secret.Type != corev1.SecretTypeTLS {
		return nil, nil, fmt.Errorf("Secret %s is type %q, expected %q", key, secret.Type, corev1.SecretTypeTLS)
	}

	certPEM, ok := secret.Data["tls.crt"]
	if !ok || len(certPEM) == 0 {
		return nil, nil, fmt.Errorf("Secret %s missing tls.crt", key)
	}

	keyPEM, ok := secret.Data["tls.key"]
	if !ok || len(keyPEM) == 0 {
		return nil, nil, fmt.Errorf("Secret %s missing tls.key", key)
	}

	return certPEM, keyPEM, nil
}

func (r *CertificateReconciler) handleCreate(ctx context.Context, log logr.Logger, cr *v1alpha1.Certificate, xc xcclient.XCClient, xcNS string, certPEM, keyPEM []byte) (ctrl.Result, error) {
	create := buildCertificateCreate(cr, xcNS, certPEM, keyPEM)
	result, err := xc.CreateCertificate(ctx, xcNS, create)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "create")
	}

	log.Info("created XC certificate", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonCreateSucceeded, "XC certificate created", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *CertificateReconciler) handleUpdate(ctx context.Context, log logr.Logger, cr *v1alpha1.Certificate, xc xcclient.XCClient, xcNS string, current *xcclient.Certificate, certPEM, keyPEM []byte) (ctrl.Result, error) {
	desiredJSON, err := buildCertificateDesiredSpecJSON(cr, xcNS, certPEM, keyPEM)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building desired spec JSON: %w", err)
	}

	needsUpdate, err := xc.ClientNeedsUpdate(current.RawSpec, desiredJSON)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("comparing specs: %w", err)
	}

	if !needsUpdate {
		r.setStatus(cr, true, true, v1alpha1.ReasonUpToDate, "XC certificate is up to date", current)
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	replace := buildCertificateReplace(cr, xcNS, current.Metadata.ResourceVersion, certPEM, keyPEM)
	result, err := xc.ReplaceCertificate(ctx, xcNS, cr.Name, replace)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "update")
	}

	log.Info("updated XC certificate", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonUpdateSucceeded, "XC certificate updated", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *CertificateReconciler) handleDeletion(ctx context.Context, log logr.Logger, cr *v1alpha1.Certificate) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, v1alpha1.FinalizerXCCleanup) {
		return ctrl.Result{}, nil
	}

	policy := cr.Annotations[v1alpha1.AnnotationDeletionPolicy]
	if policy != v1alpha1.DeletionPolicyOrphan {
		xcNS := cr.Spec.XCNamespace
		xc := r.ClientSet.Get()
		err := xc.DeleteCertificate(ctx, xcNS, cr.Name)
		if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
			return r.handleXCError(ctx, log, cr, err, "delete")
		}
		log.Info("deleted XC certificate", "name", cr.Name, "xcNamespace", xcNS)
	} else {
		log.Info("orphaning XC certificate", "name", cr.Name)
	}

	controllerutil.RemoveFinalizer(cr, v1alpha1.FinalizerXCCleanup)
	if err := r.Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *CertificateReconciler) handleXCError(ctx context.Context, log logr.Logger, cr *v1alpha1.Certificate, err error, operation string) (ctrl.Result, error) {
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

func (r *CertificateReconciler) setStatus(cr *v1alpha1.Certificate, ready, synced bool, reason, message string, xcObj *xcclient.Certificate) {
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

func (r *CertificateReconciler) setCondition(cr *v1alpha1.Certificate, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: cr.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func (r *CertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Certificate{}).
		Complete(r)
}
