resource "aws_key_pair" "filecoin" {
  key_name   = "filecoin"
  public_key = "${var.public_key}"
}

resource "aws_key_pair" "c5-gmasgras" {
  key_name   = "C5-gmasgras"
  public_key = "${var.gmasgras-public_key}"
}

resource "aws_security_group" "filecoin" {
  name   = "filecoin"
  vpc_id = "${module.vpc.vpc_id}"

  ingress {
    protocol    = "tcp"
    from_port   = 22
    to_port     = 22
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    protocol    = "tcp"
    from_port   = 9000
    to_port     = 9009
    cidr_blocks = ["0.0.0.0/0"]
    description = "swarm addresses"
  }

  ingress {
    protocol    = "tcp"
    from_port   = 8000
    to_port     = 8000
    cidr_blocks = ["0.0.0.0/0"]
    description = "block explorer UI"
  }

  ingress {
    protocol    = "tcp"
    from_port   = 8010
    to_port     = 8010
    cidr_blocks = ["0.0.0.0/0"]
    description = "dashboard UI"
  }

  ingress {
    protocol    = "tcp"
    from_port   = 19000
    to_port     = 19000
    cidr_blocks = ["0.0.0.0/0"]
    description = "dashboard aggregator collector"
  }

  ingress {
    protocol    = "tcp"
    from_port   = 9080
    to_port     = 9080
    cidr_blocks = ["0.0.0.0/0"]
    description = "dashboard aggregator Websocket"
  }

  ingress {
    protocol    = "tcp"
    from_port   = 9797
    to_port     = 9797
    cidr_blocks = ["0.0.0.0/0"]
    description = "faucet"
  }

  ingress {
    protocol    = "tcp"
    from_port   = 34530
    to_port     = 34530
    cidr_blocks = ["0.0.0.0/0"]
    description = "filecoin-0 API"
  }

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# kh test instance
module "kh-test" {
  source = "../../../modules/aws/ec2/"

  docker_tag      = "${var.docker_tag}"
  instance_type   = "c5d.4xlarge"
  instance_name   = "test"
  public_key_name = "${aws_key_pair.filecoin.key_name}"
  vpc_id          = "${module.vpc.vpc_id}"
  subnet_id       = "${element(module.vpc.public_subnets, 1)}"

  vpc_security_group_ids = [
    "${aws_security_group.filecoin.id}",
    "${aws_security_group.cadvisor.id}",
    "${aws_security_group.node_exporter.id}",
  ]

  iam_instance_profile_name = "${aws_iam_instance_profile.filecoin_kittyhawk.name}"
  route53_zone_name         = "${aws_route53_zone.kittyhawk.name}"
  route53_zone_id           = "${aws_route53_zone.kittyhawk.zone_id}"
  logstash_hosts            = "${aws_route53_record.logstash_nlb.fqdn}"
}


output "kh-test-public_ip" {
  value = "${module.kh-test.instance_public_ip}"
}

output "kh-test-dns" {
  value = "${module.kh-test.instance_dns}"
}

# output "kh-test-user_data" {
#   value = "${module.kh-test.user_data}"
# }
