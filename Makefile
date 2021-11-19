cborgen:
	cd venus-devtool && go run ./cborgen/*.go

test-venus-shared:
	cd venus-shared && go test -covermode=set ./...

test: test-venus-shared

compatible-all: compatible-actor

compatible-actor: actor-template actor-render

actor-template:
	cd venus-devtool && go run ./compatible/actors/*.go templates --dst ../venus-shared/actors/ > ../venus-shared/compatible-checks/actor-template.txt

actor-render:
	cd venus-devtool && go run ./compatible/actors/*.go render ../venus-shared/actors/
