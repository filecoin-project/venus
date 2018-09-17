variable "domain" {
  default = "filecoin-logs"
}
data "aws_region" "current" {}

data "aws_caller_identity" "current" {}

locals {
  ip_whitelist = [
    "${var.ip_whitelist}",
    "${module.vpc.nat_public_ips}"
  ]
}

resource "aws_elasticsearch_domain" "filecoin-logs" {
  domain_name           = "${var.domain}"
  elasticsearch_version = "6.3"
  cluster_config {
    instance_type = "m4.large.elasticsearch"
    instance_count = 2
  }

  ebs_options{
    ebs_enabled = true
    volume_type = "gp2"
    volume_size = 100
  }

  # useful if/when we decide to use a reverse proxy
  # https://www.elastic.co/guide/en/elasticsearch/reference/1.4/url-access-control.html
  advanced_options {
    "rest.action.multi.allow_explicit_index" = "true"
  }

  access_policies = <<CONFIG
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "es:*",
            "Principal": "*",
            "Effect": "Allow",
            "Resource": "arn:aws:es:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:domain/${var.domain}/*",
            "Condition": {
                "IpAddress": {"aws:SourceIp": ${jsonencode(local.ip_whitelist)}}
            }
        }
    ]
}
CONFIG

  snapshot_options {
    automated_snapshot_start_hour = 23
  }

  tags {
    Domain = "${var.domain}"
  }
}

output "filecoin-logs-kibana-endpoint" {
  value = "${aws_elasticsearch_domain.filecoin-logs.kibana_endpoint}"
}
