# atomic-null main.tf: null_resource + random_id (Terraform built-in providers).
# No cloud API calls, no provider downloads needed.

resource "random_id" "demo" {
  byte_length = 8
}

resource "null_resource" "demo" {
  triggers = {
    name      = var.instance_name
    ttl       = var.ttl
    secret    = var.secret_key
    vswitch   = var.vswitch_id
    random    = random_id.demo.hex
  }
}
