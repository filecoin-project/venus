resource "aws_ecr_repository" "filecoin" {
  name = "filecoin"
}

output "ecr-filecoin-url" {
  value = "${aws_ecr_repository.filecoin.repository_url}"  
}
