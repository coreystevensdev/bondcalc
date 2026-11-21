resource "aws_security_group" "ec2" {
  name        = "bondcalc-ec2-sg"
  description = "Allow API traffic; SSH is not opened, use SSM Session Manager"
  vpc_id      = data.aws_vpc.default.id

  # Caddy terminates TLS on the instance and reaches every backend over loopback,
  # so 80 and 443 are the only ports that need to be reachable. 80 exists to serve
  # the ACME challenge and to redirect to 443.
  #
  # 8080 used to be open here, from before TLS moved onto the box, and it served
  # the API in plaintext to anyone who asked. Closed 2026-11-19.
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_ecr_repository" "api" {
  name                 = "bondcalc"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_lifecycle_policy" "api" {
  repository = aws_ecr_repository.api.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Expire untagged images after 14 days"
      selection = {
        tagStatus   = "untagged"
        countType   = "sinceImagePushed"
        countUnit   = "days"
        countNumber = 14
      }
      action = { type = "expire" }
    }]
  })
}

# Standard-tier SecureString: free, encrypted with the account's default aws/ssm key.
resource "aws_ssm_parameter" "jwt_secret" {
  name  = "/bondcalc/jwt-secret"
  type  = "SecureString"
  value = var.jwt_secret

  lifecycle {
    ignore_changes = [value] # rotate out-of-band, don't let a stale tfvars stomp it
  }
}

resource "aws_iam_role" "ec2" {
  name = "bondcalc-ec2-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecr_pull" {
  role       = aws_iam_role.ec2.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "ssm_managed" {
  role       = aws_iam_role.ec2.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy" "read_jwt_secret" {
  name = "read-jwt-secret"
  role = aws_iam_role.ec2.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = aws_ssm_parameter.jwt_secret.arn
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt"]
        Resource = "arn:aws:kms:${var.aws_region}:${data.aws_caller_identity.current.account_id}:alias/aws/ssm"
      }
    ]
  })
}

resource "aws_iam_instance_profile" "ec2" {
  name = "bondcalc-ec2-profile"
  role = aws_iam_role.ec2.name
}

resource "aws_instance" "api" {
  ami                    = data.aws_ami.amazon_linux.id
  instance_type          = "t3.micro" # Free tier eligible; t2.micro is not, on newer accounts.
  iam_instance_profile   = aws_iam_instance_profile.ec2.name
  vpc_security_group_ids = [aws_security_group.ec2.id]
  subnet_id              = data.aws_subnets.default.ids[0]

  metadata_options {
    http_tokens   = "required" # IMDSv2 only
    http_endpoint = "enabled"
  }

  # deploy.sh pulls the JWT secret from SSM at run time so it never appears
  # in user_data, an SSM command string, or CloudTrail.
  user_data = base64encode(<<-EOF
    #!/bin/bash
    yum install -y docker
    systemctl enable docker
    systemctl start docker

    cat <<'SCRIPT' > /usr/local/bin/deploy.sh
    #!/bin/bash
    set -euo pipefail
    IMAGE="$1"
    JWT_SECRET=$(aws ssm get-parameter --name /bondcalc/jwt-secret --with-decryption \
      --region ${var.aws_region} --query Parameter.Value --output text)

    aws ecr get-login-password --region ${var.aws_region} | \
      docker login --username AWS --password-stdin ${aws_ecr_repository.api.repository_url}

    docker pull "$IMAGE"
    docker stop api || true
    docker rm api || true
    docker run -d \
      --name api \
      --restart=always \
      -p 127.0.0.1:8080:8080 \
      -e "JWT_SECRET=$JWT_SECRET" \
      -e "GIN_MODE=release" \
      "$IMAGE"
    SCRIPT
    chmod +x /usr/local/bin/deploy.sh

    /usr/local/bin/deploy.sh "${var.ecr_image_uri}"
    EOF
  )

  tags = { Name = "bondcalc-api" }

  # ignore_changes on user_data makes the script above a record of how this instance
  # was built, not what it runs. The box still has the pre-2026-11-19 deploy.sh, so
  # the container publishes 8080 on 0.0.0.0 and the security group above is the only
  # thing closing it. Editing the -p line here changes nothing by itself: the
  # instance has to be replaced, or the script pushed out of band.
  lifecycle {
    ignore_changes = [user_data, ami]
  }
}

# The instance's auto-assigned public IP changes if it is ever replaced, and the
# README links that address. An EIP pins it. No extra cost: AWS bills every public
# IPv4 whether auto-assigned or elastic, and attaching this releases the other, so
# the instance still has exactly one.
resource "aws_eip" "api" {
  instance = aws_instance.api.id
  domain   = "vpc"

  tags = {
    Name = "bondcalc-api"
  }
}
