resource "aws_iam_user" "circleci" {
  name = "circleci"
}

# !!! The secret access key will be written to the state file !!!
# resource "aws_iam_access_key" "circleci" {
#   user = "${aws_iam_user.circleci.name}"
# }

resource "aws_iam_user_group_membership" "circleci-ecr-users" {
  user = "${aws_iam_user.circleci.name}"

  groups = [
    "${aws_iam_group.ecr-users.name}",
  ]
}

resource "aws_iam_user_policy_attachment" "circleci_deploy-cluster" {
  user = "${aws_iam_user.circleci.name}"
  policy_arn = "${aws_iam_policy.deploy-cluster.arn}"
}

resource "aws_iam_policy" "deploy-cluster" {
  name = "deploy-cluster"
  description = "Allow deploying Kittyhawk cluster"
  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "ec2:DescribeInstances",
                "ec2:UnmonitorInstances",
                "ec2:ModifyVolumeAttribute",
                "ec2:DescribeAggregateIdFormat",
                "ec2:DescribeVolumesModifications",
                "ec2:GetHostReservationPurchasePreview",
                "ec2:DescribeSnapshots",
                "ec2:ReportInstanceStatus",
                "ec2:DescribeHostReservationOfferings",
                "ec2:ReplaceIamInstanceProfileAssociation",
                "elasticloadbalancing:DescribeLoadBalancers",
                "ec2:DeleteVolume",
                "ec2:GetLaunchTemplateData",
                "ec2:DescribeVolumeStatus",
                "ec2:StartInstances",
                "ec2:DescribeScheduledInstanceAvailability",
                "ec2:DescribeVolumes",
                "ec2:DescribeExportTasks",
                "ec2:DescribeAccountAttributes",
                "ec2:DescribeNetworkInterfacePermissions",
                "ec2:UnassignPrivateIpAddresses",
                "ec2:DescribeKeyPairs",
                "ec2:DescribeRouteTables",
                "ec2:DescribeEgressOnlyInternetGateways",
                "ec2:DetachVolume",
                "ec2:ModifyVolume",
                "ec2:DescribeVpcClassicLinkDnsSupport",
                "ec2:CreateTags",
                "ec2:DescribeSnapshotAttribute",
                "ec2:DescribeVpcPeeringConnections",
                "autoscaling:DescribeTags",
                "ec2:ResetNetworkInterfaceAttribute",
                "ec2:ModifyNetworkInterfaceAttribute",
                "ec2:DescribeIdFormat",
                "ec2:DeleteNetworkInterface",
                "ec2:RunInstances",
                "ec2:DescribeVpcEndpointServiceConfigurations",
                "autoscaling:DescribeLoadBalancers",
                "ec2:StopInstances",
                "ec2:AssignPrivateIpAddresses",
                "ec2:DescribePrefixLists",
                "ec2:DescribeVolumeAttribute",
                "ec2:DescribeInstanceCreditSpecifications",
                "ec2:DescribeVpcClassicLink",
                "ec2:CreateVolume",
                "ec2:DescribeImportSnapshotTasks",
                "elasticloadbalancing:DescribeLoadBalancerAttributes",
                "ec2:CreateNetworkInterface",
                "ec2:DescribeVpcEndpointServicePermissions",
                "ec2:DescribeScheduledInstances",
                "ec2:DisassociateIamInstanceProfile",
                "ec2:DescribeImageAttribute",
                "ec2:DescribeVpcEndpoints",
                "ec2:DescribeSubnets",
                "ec2:AttachVolume",
                "ec2:DescribeMovingAddresses",
                "ec2:DescribePrincipalIdFormat",
                "ec2:DescribeAddresses",
                "ec2:DescribeInstanceAttribute",
                "ec2:DescribeRegions",
                "ec2:DescribeFlowLogs",
                "ec2:DescribeDhcpOptions",
                "ec2:DescribeVpcEndpointServices",
                "ec2:DescribeVpcAttribute",
                "ec2:GetConsoleOutput",
                "ec2:DeleteNetworkInterfacePermission",
                "ec2:DescribeNetworkInterfaces",
                "ec2:DescribeAvailabilityZones",
                "ec2:DescribeNetworkInterfaceAttribute",
                "ec2:CreateSnapshot",
                "ec2:DescribeVpcEndpointConnections",
                "ec2:ModifyInstanceAttribute",
                "ec2:DescribeInstanceStatus",
                "ec2:TerminateInstances",
                "ec2:DetachNetworkInterface",
                "ec2:DescribeIamInstanceProfileAssociations",
                "elasticloadbalancing:DescribeTags",
                "ec2:DescribeTags",
                "ec2:DescribeBundleTasks",
                "ec2:DescribeIdentityIdFormat",
                "ec2:DescribeClassicLinkInstances",
                "ec2:DescribeCustomerGateways",
                "ec2:DescribeVpcEndpointConnectionNotifications",
                "ec2:DescribeSecurityGroups",
                "ec2:DescribeImages",
                "ec2:DescribeSecurityGroupReferences",
                "ec2:DescribeVpcs",
                "ec2:AttachNetworkInterface",
                "ec2:DescribeConversionTasks",
                "ec2:AssociateIamInstanceProfile",
                "ec2:DescribeStaleSecurityGroups"
            ],
            "Resource": "*"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": [
                "route53:GetChange",
                "dynamodb:DeleteItem",
                "route53:GetHostedZone",
                "route53:GetHealthCheck",
                "s3:ListBucket",
                "route53:UpdateHealthCheck",
                "iam:ListInstanceProfilesForRole",
                "iam:PassRole",
                "iam:ListSSHPublicKeys",
                "route53:CreateHealthCheck",
                "iam:ListAttachedRolePolicies",
                "route53:ListResourceRecordSets",
                "dynamodb:GetItem",
                "iam:ListRolePolicies",
                "iam:ListAccessKeys",
                "iam:GetRole",
                "dynamodb:BatchGetItem",
                "iam:GetInstanceProfile",
                "dynamodb:PutItem",
                "iam:GetSSHPublicKey",
                "route53:ChangeResourceRecordSets",
                "dynamodb:UpdateItem",
                "iam:ListInstanceProfiles",
                "route53:ListTagsForResource",
                "s3:PutObject",
                "s3:GetObject",
                "iam:GetRolePolicy",
                "dynamodb:GetRecords"
            ],
            "Resource": [
                "arn:aws:route53:::healthcheck/*",
                "arn:aws:route53:::change/*",
                "arn:aws:route53:::hostedzone/Z4QUK41V3HPV5",
                "arn:aws:iam::657871693752:role/filecoin_kittyhawk",
                "arn:aws:iam::657871693752:instance-profile/filecoin_kittyhawk",
                "arn:aws:iam::657871693752:user/circleci",
                "arn:aws:dynamodb:us-east-1:657871693752:table/filecoin-terraform-state",
                "arn:aws:dynamodb:us-east-1:657871693752:table/filecoin-terraform-state/stream/*",
                "arn:aws:s3:::filecoin-terraform-state",
                "arn:aws:s3:::filecoin-terraform-state/filecoin-us-east-1.tfstate"
            ]
        }
    ]
}
EOF
}
