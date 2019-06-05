---
name: Bug report
about: Create a report to help us improve
title: "[bug]"
labels: C-bug, candidate
assignees: ''

---
<!--
Please first see README for how to get help before filing a new bug report. 

If you have a *QUESTION* about Filecoin, please ask on our forum at https://discuss.filecoin.io/
-->

**Describe the bug**
<!-- A clear and concise description of what the bug is. -->

**To Reproduce**
<!--
Steps to reproduce the behavior:
1. Go to '...'
2. Run this command '...'
3. See error
-->

**Expected behavior**
<!-- A clear and concise description of what you expected to happen. -->

**Screenshots**
<!-- If applicable, add logging output or screenshots to help explain your problem. -->

<!-- Please fill out this information below completely. It will help us solve your issue faster. -->
**Diagnostic information**
<!-- If you are having issues running go-filecoin, please include the following: -->
- Filecoin Inspect Output: <!-- go-filecoin inspect all -->
- Current Head: <!--go-filecoin chain head -->
- Connected Peers: <!-- go-filecoin swarm connect -->

**Version information**
<!-- If you are having issue building go-filecoin please include the following: -->
- Go: <!-- go version -->
- Rust: <!-- rustc --version -->
- Cargo: <!-- cargo --version -->
- Commit: <!--  git rev-parse HEAD -->
- Go Env: <!-- go env -->
- CC: <!-- $(go env | grep -E "^CC" |  sed -e 's/.*\=//' | sed -e 's/^"//' -e 's/"$//') --version -->




**Additional context**
<!-- Add any other context about the problem here. -->
