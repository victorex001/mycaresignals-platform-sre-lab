output "alb_dns_name" {
  value = aws_lb.alb.dns_name
}

output "listener_arn" {
  value = aws_lb_listener.http.arn
}

output "target_group_arns" {
  value = { for k, tg in aws_lb_target_group.tg : k => tg.arn }
}