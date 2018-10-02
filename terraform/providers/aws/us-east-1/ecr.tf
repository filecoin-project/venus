resource "aws_ecr_repository" "filecoin" {
  name = "filecoin"
}

resource "aws_ecr_repository" "filebeat" {
  name = "filebeat"
}

resource "aws_ecr_repository" "logstash" {
  name = "logstash"
}

resource "aws_ecr_repository" "prometheus" {
  name = "prometheus"
}

resource "aws_ecr_repository" "alertmanager" {
  name = "alertmanager"
}

resource "aws_ecr_repository" "blockexplorer" {
  name = "blockexplorer"
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

output "ecr-prometheus-url" {
  value = "${aws_ecr_repository.prometheus.repository_url}"
}

output "ecr-alertmanager-url" {
  value = "${aws_ecr_repository.alertmanager.repository_url}"
}

output "ecr-blockexplorer-url" {
  value = "${aws_ecr_repository.blockexplorer.repository_url}"
}
