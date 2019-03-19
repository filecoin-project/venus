<!--
 This document is formatted one-sentence-per-line, breaking very long sentences at phrase boundaries.
 This format makes diffs clean and review comments easy to target.
-->

# Releasing and Deploying go-filecoin

Building, releasing and deploying go-filecoin binaries is handled entirely by CI (CircleCI).

There are two primary release and deploy paths, `User` and `Nightly`.

Linux and MacOS binaries are built and distributed in a Github release for each. For Nightly, the release is marked as a pre-release.
Additionally, a Docker image is published to ECR containing fixtures and rust-fil-proofs groth parameters.

The binary version is passed as a parameter to trigger the `go-filecoin-infra` [Nightly Devnet deploy](https://github.com/filecoin-project/go-filecoin-infra/blob/filecoin-nightly/.circleci/config.yml) or [Test Devnet deploy](https://github.com/filecoin-project/go-filecoin-infra/blob/filecoin-testnet/.circleci/config.yml).
@TODO replace 'Test Devnet' with 'User Devnet' after testing

Badges can be found in go-filecoin README.md listing current releases.

## User

The User release channel is intended as a quasi-stable (no SLA or guarantees at this time) release and testing bed
for Filecoin Developers, Infrastructure Engineers and Testers.

A release and deploy to the User Devnet is triggered by pushing a git tag meeting the following regex patterns
- `/^\d+\.\d+\.\d+$/`
- `/^testnet\-\d+\.\d+\.\d+$/ # @TODO remove this after deploy testing complete`

### Deploy instructions

1. Create a release branch eg. `release-0.1.0` based on the commit SHA of a test nightly release you would like to promote.
The nightly release should be tested prior to this.
	```
	git branch release-0.1.0 <sha1-of-nightly-release-commit>
	```
	The branch name should not match the tag versioning schema listed above.
	All subsequent work related to the release should be done here, while keeping master branch free for further development.

2. Create and push a git tag conforming to the expected tag schema listed above
	```
	git tag -a testnet-0.1.0 <sha1-of-commit>
	OR
	git tag -a 0.1.0 <sha1-of-commit>
	git push origin testnet-0.1.0
	```
	This will commence the release and deploy process in [CircleCI project](https://circleci.com/gh/filecoin-project/go-filecoin)

3. After the `go-filecoin` build successfully completes in CircleCI, you must manually approve the [Terraform deploy workflow](https://circleci.com/gh/filecoin-project/workflows/go-filecoin-infra/tree/filecoin-testnet).
View the most recent workflow and approve the `hold` job which will subsequently trigger the `deploy_user_devnet`.
This manual approval gate is to prevent unintentional destructive deployments of the User Devnet.
@TODO replace the link above with `filecoin-usernet` branch after testing

That's it! The release and deploy process should be complete.

During release testing, bug fixes or changes should be cherry picked from master in to the release branch as needed.
When releasing and deploying bug fixes or changes from the release branch, create and push a new git tag with incremented patch version ie. `0.1.1`

While release is still in `0.x.x`, it is acceptable for minor version bumps to contain breaking changes. Full semantic versioning will only be enforced on MainNet launch when the version will become `1.0.0.`

*NOTE:* CircleCI build is configured to deploy to the Test Devnet rather than the User Devnet to allow a dry run
before formally introducing new release and deploy process to User Devnet

 ## Nightly

The Nightly release channel is intended to allow daily iteration and testing without impacting the more stable User Devnet.

Triggered at 6:00 UTC daily in CircleCI scheduled job, no human intervention is needed to trigger the nightly release and deploy to the Nightly Devnet.

The following versioning scheme is utilized in Nightly
```
nightly-{circle-build-num}-{short-sha}
```
