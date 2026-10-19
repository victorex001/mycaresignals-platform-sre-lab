locals {
  # keep TG name <= 32 chars; include tg key to keep uniqueness
  tg_name = {
    for k, v in var.target_groups :
    k => substr("${var.project_name}-${var.env}-${k}", 0, 32)
  }
}

resource "aws_lb" "alb" {
  name               = substr("${var.project_name}-${var.env}-alb", 0, 32)
  load_balancer_type = "application"
  security_groups    = [var.alb_sg_id]
  subnets            = var.public_subnet_ids
}

resource "aws_lb_target_group" "tg" {
  for_each    = var.target_groups
  name        = local.tg_name[each.key]
  port        = each.value.port
  protocol    = "HTTP"
  vpc_id      = var.vpc_id
  target_type = "ip" # ✅ required for Fargate

  health_check {
    path    = each.value.health_path
    matcher = each.value.matcher
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.alb.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.tg[var.default_tg_key].arn
  }
}

resource "aws_lb_listener_rule" "rules" {
  for_each = {
    for r in var.listener_rules : tostring(r.priority) => r
  }

  listener_arn = aws_lb_listener.http.arn
  priority     = each.value.priority

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.tg[each.value.tg_key].arn
  }

  condition {
    path_pattern {
      values = each.value.path_patterns
    }
  }
}