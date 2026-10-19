data "aws_availability_zones" "available" {}

locals {
  azs = data.aws_availability_zones.available.names

  # NAT option B: single NAT gateway in ONE public subnet.
  # If nat_public_subnet_key is not set, default to the first key (stable ordering).
  public_subnet_keys  = sort(keys(aws_subnet.public))
  nat_host_subnet_key = coalesce(var.nat_public_subnet_key, local.public_subnet_keys[0])
}

# --------------------
# VPC + IGW
# --------------------
resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${var.project_name}-${var.env}-vpc"
  }
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.project_name}-${var.env}-igw"
  }
}

# --------------------
# Subnets
# --------------------
resource "aws_subnet" "public" {
  for_each = {
    a = { cidr = var.public_subnet_cidrs[0], az = local.azs[0] }
    b = { cidr = var.public_subnet_cidrs[1], az = local.azs[1] }
  }

  vpc_id                  = aws_vpc.this.id
  cidr_block              = each.value.cidr
  availability_zone       = each.value.az
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.project_name}-${var.env}-public-${each.key}"
    Tier = "public"
  }
}

resource "aws_subnet" "private" {
  for_each = {
    a = { cidr = var.private_subnet_cidrs[0], az = local.azs[0] }
    b = { cidr = var.private_subnet_cidrs[1], az = local.azs[1] }
  }

  vpc_id            = aws_vpc.this.id
  cidr_block        = each.value.cidr
  availability_zone = each.value.az

  tags = {
    Name = "${var.project_name}-${var.env}-private-${each.key}"
    Tier = "private"
  }
}

# --------------------
# NAT Gateway (SINGLE)
# --------------------
resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${var.project_name}-${var.env}-nat-eip"
  }
}

resource "aws_nat_gateway" "nat" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[local.nat_host_subnet_key].id

  depends_on = [aws_internet_gateway.igw]

  tags = {
    Name = "${var.project_name}-${var.env}-nat"
  }
}

# --------------------
# Route Tables
# --------------------
resource "aws_route_table" "public" {
  for_each = aws_subnet.public

  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }

  tags = {
    Name = "${var.project_name}-${var.env}-public-rt-${each.key}"
    Tier = "public"
  }
}

resource "aws_route_table_association" "public" {
  for_each = aws_subnet.public

  subnet_id      = each.value.id
  route_table_id = aws_route_table.public[each.key].id
}
resource "aws_route_table" "private" {
  for_each = aws_subnet.private
  vpc_id   = aws_vpc.this.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.nat.id
  }

  tags = {
    Name = "${var.project_name}-${var.env}-private-rt-${each.key}"
    Tier = "private"
  }
}


resource "aws_route_table_association" "private" {
  for_each = aws_subnet.private

  subnet_id      = each.value.id
  route_table_id = aws_route_table.private[each.key].id
}