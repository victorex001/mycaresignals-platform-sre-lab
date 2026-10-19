

# Render user-data from template (keeps image dynamic via var.image)
locals {
  user_data = templatefile("${path.module}/user_data.sh.tftpl", {
    image      = var.image
    db_host    = var.db_host
    db_port    = var.db_port
    app_port   = var.app_port
    aws_region = var.aws_region

  })
}

resource "aws_launch_template" "lt" {
  name_prefix   = "${var.project_name}-${var.env}-lt-"
  image_id      = var.ami_id
  instance_type = var.instance_type

  iam_instance_profile {
    name = var.iam_instance_profile
  }

  vpc_security_group_ids = [var.app_sg_id]

  user_data = base64encode(local.user_data)

  tag_specifications {
    resource_type = "instance"
    tags = {
      Name        = "${var.project_name}-${var.env}-app"
      Project     = var.project_name
      Environment = var.env
    }
  }

  tag_specifications {
    resource_type = "volume"
    tags = {
      Project     = var.project_name
      Environment = var.env
    }
  }
}

resource "aws_autoscaling_group" "asg" {
  name             = "${var.project_name}-${var.env}-asg"
  desired_capacity = var.desired_capacity
  min_size         = var.min_size
  max_size         = var.max_size

  vpc_zone_identifier = var.private_subnets_ids

  launch_template {
    id      = aws_launch_template.lt.id
    version = "$Latest"
  }

  # Attach to ALB target group if provided
  target_group_arns = var.target_group_arn == "" ? [] : [var.target_group_arn]

  health_check_type         = var.target_group_arn == "" ? "EC2" : "ELB"
  health_check_grace_period = 180

  termination_policies = ["OldestLaunchTemplate", "OldestInstance"]

  tag {
    key                 = "Name"
    value               = "${var.project_name}-${var.env}-asg"
    propagate_at_launch = true
  }

  tag {
    key                 = "Project"
    value               = var.project_name
    propagate_at_launch = true
  }

  tag {
    key                 = "Environment"
    value               = var.env
    propagate_at_launch = true
  }

  # Roll instances automatically when Launch Template changes (e.g., new image tag / new user_data)
  instance_refresh {
    strategy = "Rolling"
    triggers = ["launch_template"]

    preferences {
      min_healthy_percentage = 90
      instance_warmup        = 120
    }
  }
}
