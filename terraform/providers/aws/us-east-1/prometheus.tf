resource "aws_security_group" "prometheus" {
  name   = "prometheus"
  vpc_id = "${module.vpc.vpc_id}"

  ingress {
    protocol    = "tcp"
    from_port   = 22
    to_port     = 22
    cidr_blocks = ["0.0.0.0/0"]
  }

  # prometheus  
  ingress {
    protocol    = "tcp"
    from_port   = 9090
    to_port     = 9090
    cidr_blocks = ["0.0.0.0/0"]
  }

  # alertmanager  
  ingress {
    protocol    = "tcp"
    from_port   = 80
    to_port     = 80
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "cadvisor" {
  name   = "cadvisor"
  vpc_id = "${module.vpc.vpc_id}"

  ingress {
    protocol    = "tcp"
    from_port   = 8080
    to_port     = 8080
    security_groups = ["${aws_security_group.prometheus.id}"]
  }
}

resource "aws_security_group" "node_exporter" {
  name   = "node_exporter"
  vpc_id = "${module.vpc.vpc_id}"

  ingress {
    protocol    = "tcp"
    from_port   = 9100
    to_port     = 9100
    security_groups = ["${aws_security_group.prometheus.id}"]
  }
}

resource "aws_instance" "prometheus" {
  ami           = "${data.aws_ami.bionic.id}"
  key_name      = "${aws_key_pair.filecoin.key_name}"
  user_data     = "${data.template_file.prometheus_user_data.rendered}"
  instance_type = "c5d.large"

  subnet_id              = "${element(module.vpc.public_subnets, 2)}"
  vpc_security_group_ids = ["${aws_security_group.prometheus.id}"]
  iam_instance_profile   = "${aws_iam_instance_profile.prometheus.name}"

  associate_public_ip_address = true

  lifecycle {
    create_before_destroy = true
    ignore_changes = ["ami"]
  }

  tags {
    Name = "prometheus"
    metrics  = "true"
  }
}

resource "aws_route53_record" "prometheus" {
  name    = "prometheus.${aws_route53_zone.kittyhawk.name}"
  zone_id = "${aws_route53_zone.kittyhawk.zone_id}"
  type    = "A"
  records = ["${aws_instance.prometheus.public_ip}"]
  ttl     = "30"
}
resource "aws_route53_record" "alertmanager" {
  name    = "alertmanager.${aws_route53_zone.kittyhawk.name}"
  zone_id = "${aws_route53_zone.kittyhawk.zone_id}"
  type    = "A"
  records = ["${aws_instance.prometheus.public_ip}"]
  ttl     = "30"
}

data "template_file" "prometheus_user_data" {
  template = "${file("../../../scripts/prometheus_user_data.sh")}"

  vars {
    cadvisor_install = "${data.template_file.cadvisor_install.rendered}"
    node_exporter_install = "${data.template_file.node_exporter_install.rendered}"
    docker_install = "${data.template_file.docker_install.rendered}"
    alerts_slack_api_url = "${var.alerts_slack_api_url}"
    prometheus_httpasswd = "${var.prometheus_httpasswd}"
    alertmanager_httpasswd = "${var.alertmanager_httpasswd}"
  }
}

# IAM
resource "aws_iam_instance_profile" "prometheus" {
  name = "prometheus"
  role = "${aws_iam_role.prometheus.name}"
}

resource "aws_iam_role" "prometheus" {
  name = "prometheus"
  path = "/"

  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "sts:AssumeRole",
            "Principal": {
               "Service": "ec2.amazonaws.com"
            },
            "Effect": "Allow",
            "Sid": ""
        }
    ]
}
EOF
}

resource "aws_iam_policy" "ec2_allow_describe" {
  name        = "ec2_allow_describe"
  description = "Allow EC2 describe access"
  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ec2:DescribeInstances"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "prometheus-ecr_allow_all" {
  role       = "${aws_iam_role.prometheus.name}"
  policy_arn = "${aws_iam_policy.ecr_allow_all.arn}"
}

resource "aws_iam_role_policy_attachment" "prometheus-ec2_allow_describe" {
  role       = "${aws_iam_role.prometheus.name}"
  policy_arn = "${aws_iam_policy.ec2_allow_describe.arn}"
}

output "prometheus-user_data" {
  value = "${data.template_file.prometheus_user_data.rendered}"
}
