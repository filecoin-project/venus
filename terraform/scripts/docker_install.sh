while fuser /var/lib/dpkg/lock >/dev/null 2>&1 ; do
  sleep 0.5
done

passwd -d root

DEBIAN_FRONTEND=noninteractive apt-get purge -qy docker docker-engine docker.io || echo "Purged old versions of docker"
DEBIAN_FRONTEND=noninteractive apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https ca-certificates curl software-properties-common awscli golang-go jq

# add Docker gpg key and repo
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
DEBIAN_FRONTEND=noninteractive \
               add-apt-repository \
               "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
                $$(lsb_release -cs) stable"

DEBIAN_FRONTEND=noninteractive apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce
