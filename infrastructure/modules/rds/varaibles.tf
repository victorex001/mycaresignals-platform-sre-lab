variable "project_name" {}
variable "env" {}
variable "private_subnet_ids" { type = list(string) }
variable "db_sg_id" {}
variable "db_name" {}
variable "db_user" {}
variable "secret_db_password" {}
variable "db_engine_version" { default = "15" }
variable "db_instance_class" { default = "db.t4g.micro" }
