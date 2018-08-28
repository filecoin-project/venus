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

resource "aws_key_pair" "filecoin" {
  key_name   = "filecoin"
  public_key = "${var.public_key}"
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

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}

data "template_file" "user_data" {
  template = "${file("../../../scripts/docker_user_data.sh")}"

  vars {
    docker_uri = "${var.docker_uri}"
    docker_tag = "${var.docker_tag}"
  }
}

resource "aws_instance" "filecoin" {
  ami           = "${data.aws_ami.ubuntu.id}"
  key_name      = "${aws_key_pair.filecoin.key_name}"
  user_data     = "${data.template_file.user_data.rendered}"
  instance_type = "c5.2xlarge"

  subnet_id              = "${element(module.vpc.public_subnets, 0)}"
  vpc_security_group_ids = ["${aws_security_group.filecoin.id}"]
  iam_instance_profile = "${aws_iam_instance_profile.filecoin_kittyhawk.name}"

  associate_public_ip_address = true

  lifecycle {
    create_before_destroy = true
  }

  tags {
    Name = "filecoin"
  }
}

resource "aws_eip" "filecoin" {
  instance = "${aws_instance.filecoin.id}"
  vpc      = true
}

output "filecoin_dns" {
  value = "${aws_route53_record.service.fqdn}"
}

