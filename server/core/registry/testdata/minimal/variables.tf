# Minimal module fixture: 1 required string variable, no outputs.
# Used as the simplest test case for ContractExtractor.

variable "name" {
  description = "Resource name."
  type        = string
}
