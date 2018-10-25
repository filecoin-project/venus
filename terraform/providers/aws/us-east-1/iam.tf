resource "aws_iam_instance_profile" "filecoin_kittyhawk" {
  name = "filecoin_kittyhawk"
  role = "${aws_iam_role.filecoin_kittyhawk.name}"
}

resource "aws_iam_role" "filecoin_kittyhawk" {
  name = "filecoin_kittyhawk"
  path = "/"

  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "sts:AssumeRole",
            "Principal": {
               "Service": "ec2.amazonaws.com"
            },
            "Effect": "Allow",
            "Sid": ""
        }
    ]
}
EOF
}

resource "aws_iam_policy" "ecr_allow_all" {
  name        = "ecr_allow_all"
  description = "Allow ECR access"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ecr:*"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "filecoin_kittyhawk-ecr_allow_all" {
  role       = "${aws_iam_role.filecoin_kittyhawk.name}"
  policy_arn = "${aws_iam_policy.ecr_allow_all.arn}"
}


resource "aws_iam_group" "ecr-users" {
  name = "ecr-users"
}

resource "aws_iam_policy" "ecr-allow-push-pull" {
  name = "ecr-allow-push-pull"
  description = "Allow ECR Push&Pull"
  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ecr:GetDownloadUrlForLayer",
                "ecr:BatchGetImage",
                "ecr:DescribeImages",
                "ecr:GetAuthorizationToken",
                "ecr:DescribeRepositories",
                "ecr:ListImages",
                "ecr:InitiateLayerUpload",
                "ecr:BatchCheckLayerAvailability",
                "ecr:GetRepositoryPolicy",
                "ecr:PutImage",
                "ecr:UploadLayerPart",
                "ecr:CompleteLayerUpload"
            ],
            "Resource": "*"
        }
    ]
}
EOF
}

resource "aws_iam_group_policy_attachment" "ecr-users_ecr-allow-push-pull" {
  group = "${aws_iam_group.ecr-users.name}"
  policy_arn = "${aws_iam_policy.ecr-allow-push-pull.arn}"
}
