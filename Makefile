.PHONY: all run clean

all: seqblaster webcontrol

seqblaster: seqblaster.go betsy/*.go
	go build -o $@ $<

webcontrol: webcontrol.go betsy/*.go
	go build -o $@ $<

clean:
	rm -f seqblaster webcontrol
