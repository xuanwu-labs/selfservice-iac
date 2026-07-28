# Atomic RDS MySQL module fixture (W1-03 testdata).
# Covers: scalar required, scalar with default, complex default, sensitive.
# Mirrors a real alicloud RDS module's variable surface (trimmed).

variable "instance_type" {
  description = "RDS instance type (e.g. mysql.n2.large.1c)."
  type        = string
}

variable "engine_version" {
  description = "MySQL engine version."
  type        = string
  default     = "8.0"
}

variable "storage_size" {
  description = "Storage size in GB."
  type        = number
  default     = 100
}

variable "tags" {
  description = "Complex default (map) — contract MUST set default to nil per D25."
  type        = map(string)
  default = {
    env = "prod"
  }
}

variable "master_password" {
  description = "Sensitive: RDS master password (injected at runtime, not in form)."
  type        = string
  sensitive   = true
}
