To run terraform commands the AWS CLI must be [installed locally](https://docs.aws.amazon.com/cli/latest/userguide/installing.html) and a [named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-multiple-profiles.html) of `filecoin` must be configured.

Then initialize the project and check if the state file matches your local code:
``` console
$ cd terraform/providers/aws/us-east-1
$ terraform init
$ terraform plan
```

To SSH onto the machine that this project creates, find the private key in the filecoin dev 1Password vault and make a local file with that content. Be sure to set the file mode to 600 and add it to your current session with `ssh-add /path/to/key` Login to the machine with `ssh -i /path/to/key ubuntu@filecoin.kittyhawk.wtf`
