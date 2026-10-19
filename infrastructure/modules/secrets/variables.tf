variable "project_name" { type = string }
variable "env"          { type = string }
variable "service_name" { type = string }
variable "execution_role_id" { type = string }
variable "secrets" {
  type      = map(string)
  sensitive = true
}