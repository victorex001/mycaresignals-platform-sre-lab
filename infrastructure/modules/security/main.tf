

resource "aws_security_group" "alb" {
    name = "${var.project_name}-${var.env}-alb-sg"
    vpc_id = var.vpc_id

    ingress {
        from_port = 80
        to_port = 80
        protocol = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

resource "aws_security_group" "app" {
    name = "${var.project_name}=${var.env}-app-sg"
    vpc_id = var.vpc_id
    ingress {
        from_port = var.app_port
        to_port = var.app_port
        protocol = "tcp"
        security_groups = [aws_security_group.alb.id]
    }
    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

resource "aws_security_group" "db" {
    name = "${var.project_name}-${var.env}-db-sg"
    vpc_id = var.vpc_id
    ingress {
        from_port = 5432
        to_port = 5432
        protocol = "tcp"
        security_groups = [aws_security_group.app.id]
    }
    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }
}