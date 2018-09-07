resource "aws_ecr_repository" "filecoin" {
  name = "filecoin"
}

resource "aws_ecr_repository" "filebeat" {
  name = "filebeat"
}

resource "aws_ecr_repository" "logstash" {
  name = "logstash"
}


output "ecr-filecoin-url" {
  value = "${aws_ecr_repository.filecoin.repository_url}"
}
output "ecr-filebeat-url" {
  value = "${aws_ecr_repository.filebeat.repository_url}"
}
output "ecr-logstash-url" {
  value = "${aws_ecr_repository.logstash.repository_url}"
}
