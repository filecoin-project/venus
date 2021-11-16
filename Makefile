cborgen:
	cd venus-devtool && go run ./cborgen/*.go

test-venus-shared:
	cd venus-shared && go test -v -covermode=set ./...

test: test-venus-shared
