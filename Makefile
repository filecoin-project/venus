cborgen:
	cd venus-devtool && go run ./cborgen/*.go

test-venus-shared:
	cd venus-shared && go test -covermode=set ./...

test: test-venus-shared

compatible-all: compatible-actor

compatible-actor: actor-templates actor-sources actor-render

actor-templates:
	cd venus-devtool && go run ./compatible/actors/*.go templates --dst ../venus-shared/actors/ > ../venus-shared/compatible-checks/actor-templates.txt

actor-sources:
	cd venus-devtool && go run ./compatible/actors/*.go sources > ../venus-shared/compatible-checks/actor-sources.txt

actor-render:
	cd venus-devtool && go run ./compatible/actors/*.go render ../venus-shared/actors/
