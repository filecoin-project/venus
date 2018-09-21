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

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}


# instance
module "filecoin-cluster" {
  source = "../../../modules/aws/ec2/"

  instance_name = "filecoin-cluster"
  public_key_name = "${aws_key_pair.filecoin.key_name}"
  vpc_id = "${module.vpc.vpc_id}"
  subnet_id = "${element(module.vpc.public_subnets, 0)}"
  vpc_security_group_ids = [
    "${aws_security_group.filecoin.id}",
    "${aws_security_group.cadvisor.id}",
    "${aws_security_group.node_exporter.id}"
  ]
  iam_instance_profile_name = "${aws_iam_instance_profile.filecoin_kittyhawk.name}"
  route53_zone_name = "${aws_route53_zone.kittyhawk.name}"
  route53_zone_id = "${aws_route53_zone.kittyhawk.zone_id}"
}
output "filecoin-cluster-public_ip" {
  value = "${module.filecoin-cluster.instance_public_ip}"
}
output "filecoin-cluster-dns" {
  value = "${module.filecoin-cluster.instance_dns}"
}

# kh test instance
module "kh-test" {
  source = "../../../modules/aws/ec2/"

  instance_name = "kh-test"
  public_key_name = "${aws_key_pair.c5-gmasgras.key_name}"
  vpc_id = "${module.vpc.vpc_id}"
  subnet_id = "${element(module.vpc.public_subnets, 0)}"
  vpc_security_group_ids = [
    "${aws_security_group.filecoin.id}",
    "${aws_security_group.cadvisor.id}",
    "${aws_security_group.node_exporter.id}"
  ]
  iam_instance_profile_name = "${aws_iam_instance_profile.filecoin_kittyhawk.name}"
  route53_zone_name = "${aws_route53_zone.kittyhawk.name}"
  route53_zone_id = "${aws_route53_zone.kittyhawk.zone_id}"
  logstash_hosts = "${aws_route53_record.logstash_nlb.fqdn}"
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
