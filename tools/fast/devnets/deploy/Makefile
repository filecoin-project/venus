kill:
	pkill go-filecoin

clean:
	rm -rf /home/travis/tmp/filecoin/0
	rm -rf /home/travis/tmp/filecoin/1

build:
	env GO111MODULE=on go build -o deploy *.go

genesis:
	./deploy -profile genesis -profile-config=./genesis.json

bootstrap:
	./deploy -profile bootstrap -profile-config=./bootstrap1.json

