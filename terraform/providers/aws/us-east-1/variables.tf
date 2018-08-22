variable "region" {
  default = "us-east-1"
}

variable "profile" {
  default = "filecoin"
}

variable "public_key" {
  default = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQClU95JZs3J6fJSbxCivfZdZo74YssR7SMZF7PkOxPxB9+nxHEIZlZgl3t+AkliWPJSRxdPUbvzKntbuKwQ2FxlW4FYS984f0WjnfjI7ZD4qcbK2pcV47KMq/dOAC9vcTCB4uwJbigWW1yYGHbMnc+Io2Fg8fcxzhJFcJmfBs5zzsUDzrBonFiwtSqvNd3kZgPDdIQqRJosH/bT0zS5L4RaX+kqejivqq8pNYQj1oszWSyfoV21NG5V0QbKpxMQWbnGHEpFMoCW5N3lc4q+QV3Pa1BKqyCQ4Eas54c9gd6tfJKqOu0diitp9yAh+J+178HuMJuFhVWdxA7avxR7GUqhGPY8zpJi2GPcAB3hHiXU368d/k5Rkmp91U/o3S7r01eO7OOlwjaVlfoOlb1RK39SRTbqgDVw27CwoBSB4zx2wOxov8l6YsrGtbEKQjJdOZs+DSj9XsoTacWB3xYFUX6lXnaSHvKr8BK8SwR03eMsDhDBNy9pvljoH8EsRbmcpplopGPwFLwWg75R3BRSocrPnCYNsiraECwyNoDvoJt/SqSSCfsvY0o9ZemSl+JB91O/jj01gaTRM/7zppUrIjmn+xLkgg61iO9g2FFDEDsEzGFHAW9/mC00XX61TtSp9uOx7ccMxL5H4t6ZMfTaId1Oc1dloYakmPSFAOqiGJOfCQ=="
}

variable "github_branch_name" {
  default = "feat/shitty-hawk"
}

variable "datadog_api_key" {}
variable "github_token" {}

variable "es_ip_whitelist" {
  type = "list"
  default = [
    "100.9.239.66/32", #C5-LA
    "136.24.82.246/32" #eefahy
  ]  
}
