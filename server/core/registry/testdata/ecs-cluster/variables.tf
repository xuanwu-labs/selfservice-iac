# Atomic ECS cluster module fixture (W1-03 testdata).
# Covers: number with default, list(string) complex (for_each-friendly),
# bool default, region (platform-inferred, should be hidden in form).

variable "instance_count" {
  description = "Number of ECS instances (drives for_each in the real module)."
  type        = number
  default     = 3
}

variable "instance_type" {
  description = "ECS instance type."
  type        = string
  default     = "ecs.g6.large"
}

variable "image_ids" {
  description = "Complex list default — contract MUST set default to nil per D25."
  type        = list(string)
  default     = ["m-xxx", "m-yyy"]
}

variable "enable_public_ip" {
  description = "Assign public IP to instances."
  type        = bool
  default     = false
}

variable "region" {
  description = "Alicloud region (platform-inferred from env binding; hidden in form)."
  type        = string
}

variable "vpc_id" {
  description = "VPC ID (platform-inferred from env_tenant_binding; hidden in form)."
  type        = string
}
