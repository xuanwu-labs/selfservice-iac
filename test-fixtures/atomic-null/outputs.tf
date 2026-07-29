output "random_hex" {
  description = "Random hex string from random_id."
  value       = random_id.demo.hex
}

output "null_id" {
  description = "ID of the null_resource."
  value       = null_resource.demo.id
}
