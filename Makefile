SUBDIRS := $(wildcard ./lambdas/*/.)

SOURCE_FILES?=$$(go list ./... | grep -v /vendor/ | grep -v mocks)

default: lint test

all: lint $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@

lint: 
	revive -formatter friendly $(SOURCE_FILES)

.PHONY: all default lint test $(SUBDIRS)
