data "aws_availability_zones" "available" {}

module "vpc" {
  name = "filecoin"
  source  = "terraform-aws-modules/vpc/aws"
  version = "1.37.0"


  cidr                 = "10.0.0.0/16"
  public_subnets       = ["10.0.0.0/24",   "10.0.1.0/24",   "10.0.2.0/24"]

  enable_dns_support   = true
  enable_dns_hostnames = true

  azs      = ["${data.aws_availability_zones.available.names[0]}",
              "${data.aws_availability_zones.available.names[1]}",
              "${data.aws_availability_zones.available.names[2]}"]
}
