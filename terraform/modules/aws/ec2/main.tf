variable "instance_name" {}
variable "public_key_name" {}
variable "vpc_id" {}
variable "subnet_id" {}
variable "docker_uri" {
  default = "657871693752.dkr.ecr.us-east-1.amazonaws.com/filecoin"
}
variable "docker_tag" {
  default = "latest"
}
variable "filebeat_docker_uri" {
  default = "657871693752.dkr.ecr.us-east-1.amazonaws.com/filebeat"
}
variable "filebeat_docker_tag" {
  default = "latest"
}
variable "vpc_security_group_ids" { type = "list" }
variable "iam_instance_profile_name" {}
variable "route53_zone_id" {}
variable "route53_zone_name" {}
variable "logstash_hosts" {
  default = "172.17.0.1:5044" # comma separated
}

data "aws_ami" "ubuntu" {
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

resource "aws_instance" "this" {
  ami           = "${data.aws_ami.ubuntu.id}"
  key_name      = "${var.public_key_name}"
  user_data     = "${data.template_file.user_data.rendered}"
  instance_type = "r4.large"

  subnet_id              = "${var.subnet_id}"
  vpc_security_group_ids = ["${var.vpc_security_group_ids}"]
  iam_instance_profile   = "${var.iam_instance_profile_name}"

  associate_public_ip_address = true

  lifecycle {
    create_before_destroy = true
  }

  tags {
    Name     = "${var.instance_name}"
    metrics  = "true"
  }
}

data "template_file" "user_data" {
  template = "${file("${path.module}/scripts/docker_user_data.sh")}"

  vars {
    node_exporter_install = "${data.template_file.node_exporter_install.rendered}"
    cadvisor_install = "${data.template_file.cadvisor_install.rendered}"
    docker_install = "${data.template_file.docker_install.rendered}"
    docker_uri = "${var.docker_uri}"
    docker_tag = "${var.docker_tag}"
    filebeat_docker_uri = "${var.filebeat_docker_uri}"
    filebeat_docker_tag = "${var.filebeat_docker_tag}"
    logstash_hosts = "${var.logstash_hosts}"
  }
}

resource "aws_route53_record" "this" {
  name    = "${var.instance_name}.${var.route53_zone_name}"
  zone_id = "${var.route53_zone_id}"
  type    = "A"
  records = ["${aws_instance.this.public_ip}"]
  ttl     = "30"
}

output "instance_public_ip" {
  value = "${aws_instance.this.public_ip}"
}
output "instance_dns" {
  value = "${aws_route53_record.this.fqdn}"
}

output "user_data" {
  value = "${data.template_file.user_data.rendered}"
}
