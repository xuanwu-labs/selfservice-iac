// Package errors is the single runtime source of truth for platform error
// behavior. It loads the error-code registry (error-codes.yaml, embedded via
// internal/asset) at startup into an in-memory Registry, and provides typed
// helpers so handlers return structured Connect errors instead of hardcoding
// HTTP status / gRPC code / message strings.
//
// The YAML is the single source for error *behavior* (retryable, manual
// intervention, remediation, owner); proto is the single source for error
// *shape* (RPC/fields). See specs/03-平台契约.md "错误码注册表".
package errors

// Code constants reference error codes registered in error-codes.yaml.
// Handlers use these (e.g. errors.STATE_CONFLICT) instead of raw strings so
// that a typo is a compile error, not a runtime lookup failure.
//
// Keep these in sync with contracts/error-codes.yaml `code` fields. If you
// add a code to the YAML, add the constant here too.
const (
	// Validation
	CodeSchemaInvalid = "SCHEMA_INVALID"
	CodeModuleVersionNotFound = "MODULE_VERSION_NOT_FOUND"

	// Auth
	CodeUnauthenticated = "UNAUTHENTICATED"
	CodePermissionDenied = "PERMISSION_DENIED"

	// Not Found
	CodeRequestNotFound = "REQUEST_NOT_FOUND"
	CodeCatalogItemNotFound = "CATALOG_ITEM_NOT_FOUND"
	CodeArtifactNotFound = "ARTIFACT_NOT_FOUND"

	// State / Conflict
	CodeStateConflict = "STATE_CONFLICT"
	CodeIllegalStateTransition = "ILLEGAL_STATE_TRANSITION"
	CodeIdempotencyReplay = "IDEMPOTENCY_REPLAY"

	// Rate Limiting
	CodeRateLimited = "RATE_LIMITED"

	// Business Rules
	CodeBudgetExceeded = "BUDGET_EXCEEDED"
	CodePolicyViolation = "POLICY_VIOLATION"
	CodeTagMissing = "TAG_MISSING"

	// Manual Intervention
	CodeManualInterventionRequired = "MANUAL_INTERVENTION_REQUIRED"

	// Platform Errors
	CodePlatformUnavailable = "PLATFORM_UNAVAILABLE"
	CodeCloudProviderError = "CLOUD_PROVIDER_ERROR"
	CodeGitOperationFailed = "GIT_OPERATION_FAILED"
	CodeTerramateExecutionFailed = "TERRAMATE_EXECUTION_FAILED"
	CodeInternalError = "INTERNAL_ERROR"
)
