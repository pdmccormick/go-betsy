.PHONY: all run clean

all: seqblaster

seqblaster: seqblaster.go
	go build $^

clean:
	rm -f seqblaster
