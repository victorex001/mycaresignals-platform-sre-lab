packer {
  required_plugins {
    amazon = {
      source  = "github.com/hashicorp/amazon"
      version = ">= 1.0.0"
    }
  }
}

variable "aws_region" {
  type    = string
  default = "eu-west-2"
}

# Produces an AMI with Docker preinstalled
source "amazon-ebs" "ubuntu" {
  region        = var.aws_region
  instance_type = "t3.micro"
  ssh_username  = "ubuntu"

  # if you use a profile instead of env vars:
  # profile = "qr-admin"

  source_ami_filter {
    filters = {
      name                = "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"
      virtualization-type = "hvm"
      root-device-type    = "ebs"
    }
    owners      = ["099720109477"] # Canonical
    most_recent = true
  }

  # Keep a stable prefix so Terraform can find the latest by name
  ami_name = "qr-base-ubuntu-2204-docker-${formatdate("2006-01-02-1504", timestamp())}"
}

build {
  sources = ["source.amazon-ebs.ubuntu"]

  provisioner "shell" {
    inline = [
      "sudo apt-get update",
      "sudo apt-get install -y docker.io",
      "sudo systemctl enable --now docker",
      "sudo apt-get install -y git",
    ]
  }

  # Optional manifest with the AMI id
  post-processor "manifest" {
    output = "manifest.json"
  }
}