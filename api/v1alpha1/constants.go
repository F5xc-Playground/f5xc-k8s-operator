package v1alpha1

const (
	FinalizerXCCleanup = "xc.f5.com/cleanup"

	AnnotationXCNamespace    = "f5xc.io/namespace"
	AnnotationDeletionPolicy = "f5xc.io/deletion-policy"

	DeletionPolicyOrphan = "orphan"

	ConditionReady  = "Ready"
	ConditionSynced = "Synced"

	ReasonCreateSucceeded = "CreateSucceeded"
	ReasonUpdateSucceeded = "UpdateSucceeded"
	ReasonUpToDate        = "UpToDate"
	ReasonDeleteSucceeded = "DeleteSucceeded"
	ReasonCreateFailed    = "CreateFailed"
	ReasonUpdateFailed    = "UpdateFailed"
	ReasonDeleteFailed    = "DeleteFailed"
	ReasonAuthFailure     = "AuthFailure"
	ReasonRateLimited     = "RateLimited"
	ReasonServerError     = "ServerError"
	ReasonConflict        = "Conflict"
)
