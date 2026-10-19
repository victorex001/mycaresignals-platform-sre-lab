output "vpc_id" {
  value = module.network.vpc_id
}

output "public_subnet_ids" {
  value = module.network.public_subnet_ids
}

output "private_subnet_ids" {
  value = module.network.private_subnet_ids
}

output "alb_sg_id" {
  value = module.security.alb_sg_id
}

output "app_sg_id" {
  value = module.security.app_sg_id
}

output "db_sg_id" {
  value = module.security.db_sg_id
}

output "db_endpoint" {
  value = module.rds.db_endpoint
}

output "ecs_cluster_arn" {
  value = aws_ecs_cluster.this.arn
}

output "alb_dns_name" {
  value = module.alb.alb_dns_name
}

output "listener_arn" {
  value = module.alb.listener_arn
}

output "tg_arns" {
  value = module.alb.target_group_arns
}