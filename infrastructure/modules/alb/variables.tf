variable "project_name" { type = string }
variable "env"          { type = string }
variable "alb_sg_id"    { type = string }
variable "public_subnet_ids" { type = list(string) }
variable "vpc_id"       { type = string }

# Map of services -> target group config
variable "target_groups" {
  type = map(object({
    port         = number
    health_path  = string
    matcher      = string
  }))
}


# which TG gets default "/" traffic
variable "default_tg_key" {
  type = string
}

# Listener rules: list of path patterns -> forward to which TG key
variable "listener_rules" {
  type = list(object({
    priority     = number
    path_patterns = list(string)
    tg_key       = string
  }))
}