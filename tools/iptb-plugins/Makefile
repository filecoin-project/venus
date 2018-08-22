IPTB_ROOT ?= $(HOME)/testbed

all: filecoin install

install:
	mkdir -p $(IPTB_ROOT)/plugins
	cp filecoin/*.so $(IPTB_ROOT)/plugins

filecoin:
	make -C filecoin all

clean:
	rm *.so

.PHONY: all filecoin clean
