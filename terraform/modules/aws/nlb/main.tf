variable "name" {}
variable "vpc_id" {}
variable "subnets" { type = "list" }
variable "protocol" {}
variable "port" {}
variable "is_internal" {}


resource "aws_lb" "nlb" {
  name               = "${var.name}-nlb"
  load_balancer_type = "network"
  internal           = "${var.is_internal}"
  subnets            = ["${var.subnets}"]

  tags {
    Name = "${var.name}-nlb"
  }
}

resource "random_id" "target_group" {
  byte_length = 2
}

/* default target group and listeners */
resource "aws_lb_target_group" "default" {
  name                  = "${var.name}-nlb-${random_id.target_group.hex}"
  port                  = "${var.port}"
  protocol              = "${var.protocol}"
  vpc_id                = "${var.vpc_id}"
  deregistration_delay  = 30
  target_type           = "instance"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_lb_listener" "default" {
  load_balancer_arn = "${aws_lb.nlb.arn}"
  port              = "${var.port}"
  protocol          = "${var.protocol}"

  default_action {
    type = "forward"
    target_group_arn = "${aws_lb_target_group.default.id}"
  }
}

output "dns_name" {
  value = "${aws_lb.nlb.dns_name}"
}

output "zone_id" {
  value = "${aws_lb.nlb.zone_id}"
}

output "default_target_group_arn" {
  value = "${aws_lb_target_group.default.arn}"
}

output "default_listener_arn" {
  value = "${aws_lb_listener.default.arn}"
}
