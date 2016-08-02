package betsy

import (
	"bytes"
	"fmt"
	"net"
)

const PROTOCOL_PORT = 48757

type Tile struct {
	IP         net.IP
	Addr       *net.UDPAddr
	Net        *Network
	FrameBuf   bytes.Buffer
	CommandBuf bytes.Buffer
}

type Network struct {
	Conn          *net.UDPConn
	BroadcastAddr *net.UDPAddr
	Interface     *net.Interface
	CommandBuf    bytes.Buffer
}

func (network *Network) MakeTile(ip net.IP) (*Tile, error) {
	addr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s%%%s]:%d", ip.String(), network.Interface.Name, PROTOCOL_PORT))
	if err != nil {
		return nil, err
	}

	tile := &Tile{
		IP:   ip,
		Net:  network,
		Addr: addr,
	}
	return tile, nil
}

func (network *Network) BroadcastCommand(command string) error {
	network.CommandBuf.Reset()
	fmt.Fprintf(&network.CommandBuf, "%s", command)

	// Transmit command
	if _, err := network.Conn.WriteTo(network.CommandBuf.Bytes(), network.BroadcastAddr); err != nil {
		return err
	}
	return nil
}

func (network *Network) UploadFrame() error {
	return network.BroadcastCommand("dpc upload;")
}

func NetworkByInterfaceName(name string) (*Network, error) {
	const IPV6_ALL_NODES_MULTICAST_ADDRESS = "ff02::1"

	dev, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	baddr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s%%%s]:%d", IPV6_ALL_NODES_MULTICAST_ADDRESS, dev.Name, PROTOCOL_PORT))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp6", nil)
	if err != nil {
		return nil, err
	}

	return &Network{
		Conn:          conn,
		BroadcastAddr: baddr,
		Interface:     dev,
	}, nil
}

func (tile *Tile) SendFrameBuffer(frame []byte) error {
	// Break frame buffer into chunks and send individually
	const FRAME_CHUNK_SIZE = 1024
	buf := bytes.NewBuffer(frame)
	for offs := 0; buf.Len() > 0; offs += FRAME_CHUNK_SIZE {
		// Extract next chunk of the frame
		chunk := buf.Next(FRAME_CHUNK_SIZE)

		// Pack chunk into the command buffer
		tile.CommandBuf.Reset()
		fmt.Fprintf(&tile.CommandBuf, "dpc data %d;", offs)
		if _, err := tile.CommandBuf.Write(chunk); err != nil {
			return err
		}

		// Transmit command
		if _, err := tile.Net.Conn.WriteTo(tile.CommandBuf.Bytes(), tile.Addr); err != nil {
			return err
		}
	}

	return nil
}
