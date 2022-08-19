#! bash
# this script is used to pre download and install golang dependencies from source repos

go env -w GOPROXY=https://goproxy.io,direct

repos=(\
https://github.com/filecoin-project/venus.git \
https://github.com/filecoin-project/venus-messager.git \
https://github.com/ipfs-force-community/venus-gateway.git \
https://github.com/filecoin-project/venus-auth.git \
https://github.com/filecoin-project/venus-miner.git \
https://github.com/filecoin-project/venus-wallet.git \
https://github.com/filecoin-project/venus-market.git \
) 

mkdir -p /gomod/
cd /gomod/

function checkout {
    git clone $1 --depth 1
}

for repo in ${repos[@]}
do checkout $repo --depth 1
done

for directory in $(ls)
do
    if [ -d $directory ]
    then 
        cd $directory 
        go mod download -x
        cd ..
        echo $directory
    fi
done

rm -rf /gomod
