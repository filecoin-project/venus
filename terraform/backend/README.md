This Terraform project manages a DynamoDB table and an S3 bucket used to store Terraform state files remotely. For more information see: [https://www.terraform.io/docs/state/remote.html](https://www.terraform.io/docs/state/remote.html)

To check the state of these resources, the AWS CLI must be [installed locally](https://docs.aws.amazon.com/cli/latest/userguide/installing.html) and a [named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-multiple-profiles.html) of `filecoin` must be configured.
