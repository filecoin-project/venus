resource "aws_ssm_parameter" "kittyhawk-node-keys-pass" {
  name  = "kittyhawk-node-keys-pass"
  type  = "SecureString"
  value = "${var.kittyhawk-node-keys-pass}"
}
resource "aws_ssm_parameter" "kittyhawk-node-alertmanager_basic_auth" {
  name  = "kittyhawk-node-alertmanager_basic_auth"
  type  = "SecureString"
  value = "${var.kittyhawk-node-alertmanager_basic_auth}"
}
