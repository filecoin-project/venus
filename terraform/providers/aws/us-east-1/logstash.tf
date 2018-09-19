locals {
  name = "logstash"
  hosted_zone_name = "${aws_route53_zone.kittyhawk.name}"
  hosted_zone_id = "${aws_route53_zone.kittyhawk.zone_id}"
  port = "5044"
}

data "aws_ami" "bionic" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-bionic-18.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"]
}

data "template_file" "logstash_user_data" {
  template = "${file("../../../scripts/logstash_user_data.sh")}"

  vars {
    docker_install = "${data.template_file.docker_install.rendered}"
    logstash_docker_uri = "${var.logstash_docker_uri}"
    logstash_docker_tag = "${var.logstash_docker_tag}"
    es_host = "${var.logstash_es_host}"
  }
}

resource "aws_security_group" "logstash" {
  name   = "logstash"
  vpc_id = "${module.vpc.vpc_id}"

  ingress {
    protocol = "tcp"
    from_port = "${local.port}"
    to_port = "${local.port}"
    security_groups = ["${aws_security_group.filecoin.id}"]
  }

  ingress {
    protocol = "tcp"
    from_port = "${local.port}"
    to_port = "${local.port}"
    cidr_blocks = ["${module.vpc.vpc_cidr_block}"]
  }

  ingress {
    protocol    = "tcp"
    from_port   = 22
    to_port     = 22
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}
resource "aws_instance" "logstash" {
  count         = "2"
  ami           = "${data.aws_ami.bionic.id}"
  key_name      = "${aws_key_pair.c5-gmasgras.key_name}"
  user_data     = "${data.template_file.logstash_user_data.rendered}"
  instance_type = "t2.small"

  subnet_id              = "${element(module.vpc.private_subnets, count.index)}"
  vpc_security_group_ids = ["${aws_security_group.logstash.id}"]
  iam_instance_profile   = "${aws_iam_instance_profile.filecoin_kittyhawk.name}"

  lifecycle {
    create_before_destroy = true
  }

  tags {
    Name = "${local.name}-${count.index}"
  }

}

module "logstash_nlb" {
  source = "../../../modules/aws/nlb"

  name        = "${local.name}"
  is_internal = true
  vpc_id      = "${module.vpc.vpc_id}"
  subnets     = "${module.vpc.public_subnets}"
  protocol    = "TCP"
  port        = "${local.port}"
}

resource "aws_route53_record" "logstash_nlb" {
  zone_id = "${local.hosted_zone_id}"
  name    = "${local.name}.${local.hosted_zone_name}"
  type    = "A"

  alias {
    name    = "${module.logstash_nlb.dns_name}"
    zone_id = "${module.logstash_nlb.zone_id}"

    evaluate_target_health = true
  }
}

resource "aws_lb_target_group_attachment" "logstash" {
  count = "2"
  target_group_arn = "${module.logstash_nlb.default_target_group_arn}"
  target_id = "${element(aws_instance.logstash.*.id, count.index)}"
  port = "${local.port}"
  depends_on = ["aws_instance.logstash"]
}
