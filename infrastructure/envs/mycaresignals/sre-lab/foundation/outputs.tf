output "verified_account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "vpc_id" {
  value = aws_vpc.this.id
}

output "vpc_cidr" {
  value = aws_vpc.this.cidr_block
}

output "private_subnet_ids" {
  value = aws_subnet.private[*].id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_route_table_id" {
  value = aws_route_table.private.id
}

output "nat_gateway_enabled" {
  value = var.enable_nat_gateway
}

output "vpc_flow_log_group" {
  value = var.enable_vpc_flow_logs ? aws_cloudwatch_log_group.vpc_flow[0].name : null
}
