resource "aws_kms_key" "data" {
  description             = "myCareSignals SRE lab EKS secrets, RDS and result encryption"
  enable_key_rotation     = true
  deletion_window_in_days = 7

  tags = { Name = "${local.name_prefix}-data" }
}

resource "aws_kms_alias" "data" {
  name          = "alias/${local.name_prefix}-data"
  target_key_id = aws_kms_key.data.key_id
}

resource "aws_cloudwatch_log_group" "eks_control_plane" {
  name              = "/aws/eks/${local.cluster_name}/cluster"
  retention_in_days = var.log_retention_days
  kms_key_id        = aws_kms_key.data.arn
}

resource "aws_iam_role" "eks_cluster" {
  name = "${local.name_prefix}-eks-cluster"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "eks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

resource "aws_eks_cluster" "this" {
  name     = local.cluster_name
  role_arn = aws_iam_role.eks_cluster.arn
  version  = var.kubernetes_version

  enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]

  access_config {
    authentication_mode                         = "API"
    bootstrap_cluster_creator_admin_permissions = true
  }

  encryption_config {
    provider {
      key_arn = aws_kms_key.data.arn
    }
    resources = ["secrets"]
  }

  vpc_config {
    subnet_ids              = data.terraform_remote_state.foundation.outputs.private_subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = true
    public_access_cidrs     = var.operator_cidrs
  }

  depends_on = [
    aws_cloudwatch_log_group.eks_control_plane,
    aws_iam_role_policy_attachment.eks_cluster
  ]

  tags = { Name = local.cluster_name }
}

resource "aws_iam_role" "eks_nodes" {
  name = "${local.name_prefix}-eks-nodes"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_nodes" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
    "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
    "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
  ])

  role       = aws_iam_role.eks_nodes.name
  policy_arn = each.value
}

resource "aws_eks_node_group" "system" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "system-on-demand"
  node_role_arn   = aws_iam_role.eks_nodes.arn
  subnet_ids      = data.terraform_remote_state.foundation.outputs.private_subnet_ids
  instance_types  = var.system_instance_types
  capacity_type   = "ON_DEMAND"
  ami_type        = "AL2023_x86_64_STANDARD"

  scaling_config {
    min_size     = 2
    desired_size = 2
    max_size     = 4
  }

  update_config {
    max_unavailable = 1
  }

  labels = {
    workload       = "system"
    node_lifecycle = "on-demand"
  }

  depends_on = [aws_iam_role_policy_attachment.eks_nodes]
}

resource "aws_eks_node_group" "application" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "application-on-demand"
  node_role_arn   = aws_iam_role.eks_nodes.arn
  subnet_ids      = data.terraform_remote_state.foundation.outputs.private_subnet_ids
  instance_types  = var.application_instance_types
  capacity_type   = "ON_DEMAND"
  ami_type        = "AL2023_x86_64_STANDARD"

  scaling_config {
    min_size     = 2
    desired_size = 2
    max_size     = 10
  }

  update_config {
    max_unavailable_percentage = 25
  }

  labels = {
    workload       = "application"
    node_lifecycle = "on-demand"
  }

  lifecycle {
    ignore_changes = [scaling_config[0].desired_size]
  }

  depends_on = [aws_iam_role_policy_attachment.eks_nodes]
}

resource "aws_eks_node_group" "load" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "load-generator-spot"
  node_role_arn   = aws_iam_role.eks_nodes.arn
  subnet_ids      = data.terraform_remote_state.foundation.outputs.private_subnet_ids
  instance_types  = var.load_instance_types
  capacity_type   = "SPOT"
  ami_type        = "AL2023_x86_64_STANDARD"

  scaling_config {
    min_size     = 0
    desired_size = 0
    max_size     = var.load_max_nodes
  }

  update_config {
    max_unavailable_percentage = 50
  }

  labels = {
    workload       = "load"
    node_lifecycle = "spot"
  }

  taint {
    key    = "dedicated"
    value  = "load"
    effect = "NO_SCHEDULE"
  }

  lifecycle {
    ignore_changes = [scaling_config[0].desired_size]
  }

  depends_on = [aws_iam_role_policy_attachment.eks_nodes]
}

locals {
  autoscaled_node_groups = {
    system = {
      autoscaling_group_name = aws_eks_node_group.system.resources[0].autoscaling_groups[0].name
      workload               = "system"
      lifecycle              = "on-demand"
    }
    application = {
      autoscaling_group_name = aws_eks_node_group.application.resources[0].autoscaling_groups[0].name
      workload               = "application"
      lifecycle              = "on-demand"
    }
    load = {
      autoscaling_group_name = aws_eks_node_group.load.resources[0].autoscaling_groups[0].name
      workload               = "load"
      lifecycle              = "spot"
    }
  }
}

resource "aws_autoscaling_group_tag" "cluster_autoscaler_enabled" {
  for_each = local.autoscaled_node_groups

  autoscaling_group_name = each.value.autoscaling_group_name

  tag {
    key                 = "k8s.io/cluster-autoscaler/enabled"
    value               = "true"
    propagate_at_launch = false
  }
}

resource "aws_autoscaling_group_tag" "cluster_autoscaler_owned" {
  for_each = local.autoscaled_node_groups

  autoscaling_group_name = each.value.autoscaling_group_name

  tag {
    key                 = "k8s.io/cluster-autoscaler/${aws_eks_cluster.this.name}"
    value               = "owned"
    propagate_at_launch = false
  }
}

resource "aws_autoscaling_group_tag" "cluster_autoscaler_workload" {
  for_each = local.autoscaled_node_groups

  autoscaling_group_name = each.value.autoscaling_group_name

  tag {
    key                 = "k8s.io/cluster-autoscaler/node-template/label/workload"
    value               = each.value.workload
    propagate_at_launch = false
  }
}

resource "aws_autoscaling_group_tag" "cluster_autoscaler_lifecycle" {
  for_each = local.autoscaled_node_groups

  autoscaling_group_name = each.value.autoscaling_group_name

  tag {
    key                 = "k8s.io/cluster-autoscaler/node-template/label/node_lifecycle"
    value               = each.value.lifecycle
    propagate_at_launch = false
  }
}

resource "aws_autoscaling_group_tag" "cluster_autoscaler_load_taint" {
  autoscaling_group_name = aws_eks_node_group.load.resources[0].autoscaling_groups[0].name

  tag {
    key                 = "k8s.io/cluster-autoscaler/node-template/taint/dedicated"
    value               = "load:NoSchedule"
    propagate_at_launch = false
  }
}

data "tls_certificate" "eks_oidc" {
  url = aws_eks_cluster.this.identity[0].oidc[0].issuer
}

resource "aws_iam_openid_connect_provider" "eks" {
  url             = aws_eks_cluster.this.identity[0].oidc[0].issuer
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.eks_oidc.certificates[0].sha1_fingerprint]
}

data "aws_iam_policy_document" "cluster_autoscaler_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.eks.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${replace(aws_iam_openid_connect_provider.eks.url, "https://", "")}:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "${replace(aws_iam_openid_connect_provider.eks.url, "https://", "")}:sub"
      values   = ["system:serviceaccount:kube-system:cluster-autoscaler"]
    }
  }
}

resource "aws_iam_role" "cluster_autoscaler" {
  name               = "${local.name_prefix}-cluster-autoscaler"
  assume_role_policy = data.aws_iam_policy_document.cluster_autoscaler_assume.json
}

data "aws_iam_policy_document" "cluster_autoscaler" {
  statement {
    sid = "ReadScalingState"
    actions = [
      "autoscaling:DescribeAutoScalingGroups",
      "autoscaling:DescribeAutoScalingInstances",
      "autoscaling:DescribeLaunchConfigurations",
      "autoscaling:DescribeScalingActivities",
      "autoscaling:DescribeTags",
      "ec2:DescribeImages",
      "ec2:DescribeInstanceTypes",
      "ec2:DescribeLaunchTemplateVersions",
      "ec2:GetInstanceTypesFromInstanceRequirements",
      "eks:DescribeNodegroup"
    ]
    resources = ["*"]
  }

  statement {
    sid = "ScaleOnlyLabNodeGroups"
    actions = [
      "autoscaling:SetDesiredCapacity",
      "autoscaling:TerminateInstanceInAutoScalingGroup"
    ]
    resources = ["*"]

    condition {
      test     = "StringEquals"
      variable = "aws:ResourceTag/k8s.io/cluster-autoscaler/enabled"
      values   = ["true"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:ResourceTag/k8s.io/cluster-autoscaler/${aws_eks_cluster.this.name}"
      values   = ["owned"]
    }
  }
}

resource "aws_iam_role_policy" "cluster_autoscaler" {
  name   = "scale-sre-lab-node-groups"
  role   = aws_iam_role.cluster_autoscaler.id
  policy = data.aws_iam_policy_document.cluster_autoscaler.json
}

resource "aws_eks_addon" "vpc_cni" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "vpc-cni"
  resolve_conflicts_on_update = "PRESERVE"
  configuration_values = jsonencode({
    enableNetworkPolicy = "true"
  })
}

resource "aws_eks_addon" "kube_proxy" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "kube-proxy"
  resolve_conflicts_on_update = "PRESERVE"
}

resource "aws_eks_addon" "coredns" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "coredns"
  resolve_conflicts_on_update = "PRESERVE"

  depends_on = [aws_eks_node_group.system]
}

resource "aws_eks_addon" "metrics_server" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "metrics-server"
  resolve_conflicts_on_update = "PRESERVE"

  depends_on = [aws_eks_node_group.system]
}

resource "aws_eks_addon" "cloudwatch" {
  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = "amazon-cloudwatch-observability"
  resolve_conflicts_on_update = "PRESERVE"

  depends_on = [aws_eks_node_group.system]
}

resource "aws_ecr_repository" "images" {
  for_each = toset(["teapi", "teapi-migrate", "loadgen", "seed-tools"])

  name                 = "${local.name_prefix}/${each.value}"
  image_tag_mutability = "IMMUTABLE"
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "KMS"
    kms_key         = aws_kms_key.data.arn
  }
}

resource "aws_ecr_lifecycle_policy" "images" {
  for_each = aws_ecr_repository.images

  repository = each.value.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Retain the most recent ten lab images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 10
      }
      action = { type = "expire" }
    }]
  })
}
