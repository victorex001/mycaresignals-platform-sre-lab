# COST-OPTIMIZED EKS CLUSTER
# Monthly Cost: ~$162 (vs $323 original)
# - 1x t3.small On-Demand (critical/auth workloads)
# - 1x t3.small Spot (general workloads, 70% cheaper)
# - Reliable for auth, cheap for everything else

data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name_prefix = "${var.project_name}-${var.env}-${var.service_name}"
  azs         = slice(data.aws_availability_zones.available.names, 0, 2)
}

# ----------------------------
# VPC (unchanged - good architecture)
# ----------------------------
resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${local.name_prefix}-vpc"
  }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${local.name_prefix}-igw"
  }
}

resource "aws_subnet" "public" {
  count = length(var.public_subnet_cidrs)

  vpc_id                  = aws_vpc.this.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = local.azs[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name                     = "${local.name_prefix}-public-${count.index + 1}"
    "kubernetes.io/role/elb" = "1"
  }
}

resource "aws_subnet" "private" {
  count = length(var.private_subnet_cidrs)

  vpc_id            = aws_vpc.this.id
  cidr_block        = var.private_subnet_cidrs[count.index]
  availability_zone = local.azs[count.index]

  tags = {
    Name                              = "${local.name_prefix}-private-${count.index + 1}"
    "kubernetes.io/role/internal-elb" = "1"
  }
}

resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${local.name_prefix}-nat-eip"
  }
}

resource "aws_nat_gateway" "this" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id

  tags = {
    Name = "${local.name_prefix}-nat"
  }

  depends_on = [aws_internet_gateway.this]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = {
    Name = "${local.name_prefix}-public-rt"
  }
}

resource "aws_route_table_association" "public" {
  count = length(aws_subnet.public)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.this.id
  }

  tags = {
    Name = "${local.name_prefix}-private-rt"
  }
}

resource "aws_route_table_association" "private" {
  count = length(aws_subnet.private)

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

# ----------------------------
# EKS IAM
# ----------------------------
resource "aws_iam_role" "eks_cluster" {
  name = "${local.name_prefix}-eks-cluster-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "eks.amazonaws.com"
      }
      Action = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

resource "aws_iam_role" "eks_nodes" {
  name = "${local.name_prefix}-eks-node-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
      Action = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_worker_node" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}

resource "aws_iam_role_policy_attachment" "eks_cni" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
}

resource "aws_iam_role_policy_attachment" "eks_ecr_readonly" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

# ----------------------------
# EKS Cluster
# ----------------------------
resource "aws_security_group" "eks_cluster" {
  name        = "${local.name_prefix}-eks-cluster-sg"
  description = "EKS cluster security group"
  vpc_id      = aws_vpc.this.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${local.name_prefix}-eks-cluster-sg"
  }
}

resource "aws_eks_cluster" "this" {
  name     = var.cluster_name
  role_arn = aws_iam_role.eks_cluster.arn
  version  = "1.30"

  vpc_config {
    subnet_ids              = concat(aws_subnet.public[*].id, aws_subnet.private[*].id)
    endpoint_private_access = true
    endpoint_public_access  = true
    security_group_ids      = [aws_security_group.eks_cluster.id]
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy
  ]

  tags = {
    Name = var.cluster_name
  }
}

# ----------------------------
# COST OPTIMIZATION: Hybrid Node Groups
# ----------------------------

# ON-DEMAND NODE GROUP (for critical workloads: auth, payments, core API)
resource "aws_eks_node_group" "on_demand_critical" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "${local.name_prefix}-on-demand-critical"
  node_role_arn   = aws_iam_role.eks_nodes.arn
  subnet_ids      = aws_subnet.private[*].id

  # OPTIMIZED: Small instance, always available
  instance_types = ["t3.small"]
  capacity_type  = "ON_DEMAND"

  scaling_config {
    desired_size = 1 # Start with 1
    min_size     = 1 # Always keep 1 for reliability
    max_size     = 2 # Can scale to 2 if needed
  }

  # Labels to identify critical nodes
  labels = {
    workload-type  = "critical"
    node-lifecycle = "on-demand"
  }

  tags = {
    Name = "${local.name_prefix}-on-demand-critical"
    # Enable cluster autoscaler
    "k8s.io/cluster-autoscaler/enabled"             = "true"
    "k8s.io/cluster-autoscaler/${var.cluster_name}" = "owned"
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_worker_node,
    aws_iam_role_policy_attachment.eks_cni,
    aws_iam_role_policy_attachment.eks_ecr_readonly
  ]
}

# SPOT NODE GROUP (for general workloads: workers, background jobs)
resource "aws_eks_node_group" "spot_general" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "${local.name_prefix}-spot-general"
  node_role_arn   = aws_iam_role.eks_nodes.arn
  subnet_ids      = aws_subnet.private[*].id

  # OPTIMIZED: Spot instances are 60-70% cheaper!
  # Multiple instance types for better spot availability
  instance_types = ["t3.small", "t3a.small", "t2.small"]
  capacity_type  = "SPOT"

  scaling_config {
    desired_size = 1 # Start with 1
    min_size     = 0 # Can scale to 0 when not needed
    max_size     = 5 # Can scale up during high load
  }

  # Labels to identify spot nodes
  labels = {
    workload-type  = "general"
    node-lifecycle = "spot"
  }

  tags = {
    Name = "${local.name_prefix}-spot-general"
    # Enable cluster autoscaler
    "k8s.io/cluster-autoscaler/enabled"             = "true"
    "k8s.io/cluster-autoscaler/${var.cluster_name}" = "owned"
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_worker_node,
    aws_iam_role_policy_attachment.eks_cni,
    aws_iam_role_policy_attachment.eks_ecr_readonly
  ]
}

# ----------------------------
# OIDC Provider
# ----------------------------
data "tls_certificate" "eks_oidc" {
  url = aws_eks_cluster.this.identity[0].oidc[0].issuer
}

resource "aws_iam_openid_connect_provider" "this" {
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.eks_oidc.certificates[0].sha1_fingerprint]
  url             = aws_eks_cluster.this.identity[0].oidc[0].issuer
}

# ----------------------------
# AWS Load Balancer Controller
# ----------------------------
resource "aws_iam_policy" "aws_load_balancer_controller" {
  name   = "${local.name_prefix}-aws-load-balancer-controller"
  policy = file("${path.module}/iam/aws-load-balancer-controller-policy.json")
}

resource "aws_iam_role" "aws_load_balancer_controller" {
  name = "${local.name_prefix}-aws-load-balancer-controller-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Federated = aws_iam_openid_connect_provider.this.arn
      }
      Action = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "${replace(aws_iam_openid_connect_provider.this.url, "https://", "")}:sub" = "system:serviceaccount:kube-system:aws-load-balancer-controller"
          "${replace(aws_iam_openid_connect_provider.this.url, "https://", "")}:aud" = "sts.amazonaws.com"
        }
      }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "aws_load_balancer_controller" {
  role       = aws_iam_role.aws_load_balancer_controller.name
  policy_arn = aws_iam_policy.aws_load_balancer_controller.arn
}

resource "kubernetes_service_account" "aws_load_balancer_controller" {
  metadata {
    name      = "aws-load-balancer-controller"
    namespace = "kube-system"
    annotations = {
      "eks.amazonaws.com/role-arn" = aws_iam_role.aws_load_balancer_controller.arn
    }
  }

  depends_on = [
    aws_eks_node_group.on_demand_critical,
    aws_iam_role_policy_attachment.aws_load_balancer_controller
  ]
}

resource "helm_release" "aws_load_balancer_controller" {
  name       = "aws-load-balancer-controller"
  repository = "https://aws.github.io/eks-charts"
  chart      = "aws-load-balancer-controller"
  namespace  = "kube-system"
  version    = "1.14.0"

  set {
    name  = "clusterName"
    value = aws_eks_cluster.this.name
  }

  set {
    name  = "serviceAccount.create"
    value = "false"
  }

  set {
    name  = "serviceAccount.name"
    value = kubernetes_service_account.aws_load_balancer_controller.metadata[0].name
  }

  set {
    name  = "region"
    value = var.aws_region
  }

  set {
    name  = "vpcId"
    value = aws_vpc.this.id
  }

  depends_on = [
    kubernetes_service_account.aws_load_balancer_controller
  ]
}

# ----------------------------
# Cognito
# ----------------------------
resource "aws_cognito_user_pool" "this" {
  name = "${local.name_prefix}-user-pool"

  auto_verified_attributes = ["email"]
  username_attributes      = ["email"]

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  password_policy {
    minimum_length    = 12
    require_uppercase = true
    require_lowercase = true
    require_numbers   = true
    require_symbols   = true
  }
}

resource "aws_cognito_user_pool_client" "this" {
  name                                 = "${local.name_prefix}-staff-web-client"
  user_pool_id                         = aws_cognito_user_pool.this.id
  generate_secret                      = false
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["email", "openid", "profile"]
  callback_urls                        = var.staff_auth_callback_urls
  logout_urls                          = var.staff_auth_logout_urls
  supported_identity_providers         = ["COGNITO"]
  prevent_user_existence_errors        = "ENABLED"

  explicit_auth_flows = [
    "ALLOW_REFRESH_TOKEN_AUTH"
  ]
}

resource "aws_cognito_user_pool_domain" "this" {
  domain       = "${local.name_prefix}-staff"
  user_pool_id = aws_cognito_user_pool.this.id
}

resource "aws_cognito_user_group" "staff" {
  name         = "staff"
  user_pool_id = aws_cognito_user_pool.this.id
  description  = "General staff users; application RBAC still authorizes homes and children."
  precedence   = 30
}

resource "aws_cognito_user_group" "manager" {
  name         = "manager"
  user_pool_id = aws_cognito_user_pool.this.id
  description  = "Managers; application RBAC still authorizes homes and children."
  precedence   = 20
}

resource "aws_cognito_user_group" "admin" {
  name         = "admin"
  user_pool_id = aws_cognito_user_pool.this.id
  description  = "Administrators; application RBAC still authorizes homes and children."
  precedence   = 10
}

# ----------------------------
# App Secret
# ----------------------------
resource "aws_secretsmanager_secret" "app" {
  name = "App_Secrets"
}

resource "aws_secretsmanager_secret_version" "app" {
  secret_id = aws_secretsmanager_secret.app.id

  secret_string = jsonencode({
    AUTH_GRPC_PORT       = var.auth_grpc_port
    JWT_SECRET_KEY       = var.jwt_secret_key
    COGNITO_USER_POOL_ID = aws_cognito_user_pool.this.id
    COGNITO_CLIENT_ID    = aws_cognito_user_pool_client.this.id
    AWS_REGION           = var.aws_region
    POSTGRES_DSN         = var.postgres_dsn
  })
}
