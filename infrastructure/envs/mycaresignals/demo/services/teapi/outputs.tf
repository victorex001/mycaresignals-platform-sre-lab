output "ecs_service_name" {
  value = aws_ecs_service.api.name
}

output "task_definition_arn" {
  value = aws_ecs_task_definition.api.arn
}

output "migration_task_definition_arn" {
  value = aws_ecs_task_definition.migration.arn
}

output "log_group_name" {
  value = aws_cloudwatch_log_group.app.name
}

output "shared_alb_target_group_arn" {
  value = aws_lb_target_group.shared_alb.arn
}
