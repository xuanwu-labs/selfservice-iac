# atomic-null test fixture: a minimal Terraform module using built-in providers
# (null + random) for E2E testing without cloud credentials.
#
# Contains 4 field types that exercise ContractExtractor + FormSchemaGenerator:
#   - scalar required (instance_name)
#   - scalar default (ttl)
#   - sensitive (secret_key) — should be hidden from form
#   - platform-inferred (vswitch_id) — should be hidden from form

variable "instance_name" {
  description = "Name for the null resource instance (required, user-facing)."
  type        = string
}

variable "ttl" {
  description = "Time-to-live in seconds for the demo resource."
  type        = number
  default     = 300
}

variable "secret_key" {
  description = "Sensitive key — injected at runtime, NOT shown in form."
  type        = string
  sensitive   = true
}

variable "vswitch_id" {
  description = "Platform-inferred — injected via cross-layer remote_state."
  type        = string
}
