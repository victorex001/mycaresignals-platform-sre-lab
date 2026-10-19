variable "ami_id" {
  type = string
  description = "Prebaked AMI ID (from Packer)"
}

variable "project_name" {
  type = string
}

variable "env" {
  type = string
}

variable "image" {
  type = string
}

variable "instance_type" {
  type    = string
  default = "t3.micro"
}

# NOTE: keeping your original name with the extra 's'
variable "private_subnets_ids" {
  type = list(string)
}

variable "iam_instance_profile" {
  type = string
}

variable "app_sg_id" {
  type = string
}

variable "db_host" {
  type = string
}

variable "db_port" {
  type = number
  # default = 5432  # optional if you prefer set in root
}

variable "target_group_arn" {
  type        = string
  description = "Target group ARN for attaching the ASG; empty string means no attachment"
  default     = ""
}


variable "desired_capacity" {
  type    = number
  default = 1
}

variable "min_size" {
  type    = number
  default = 1
}

variable "max_size" {
  type    = number
  default = 2
}

variable "aws_region" {
  type = string
}
variable "app_port" {
  type = number
}