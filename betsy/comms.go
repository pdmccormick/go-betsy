package betsy

import (
	"bytes"
	"fmt"
	"net"
	"syscall"
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
	UnicastConn   *net.UDPConn
	BroadcastConn *net.UDPConn
	BroadcastAddr *net.UDPAddr
	Interface     *net.Interface
	CommandBuf    bytes.Buffer
}

func (network *Network) Close() {
	network.UnicastConn.Close()
	network.BroadcastConn.Close()
}

func (network *Network) MakeTile(ip net.IP) (*Tile, error) {
	addr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s]:%d", ip.String(), PROTOCOL_PORT))
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
	if _, err := network.BroadcastConn.WriteTo(network.CommandBuf.Bytes(), network.BroadcastAddr); err != nil {
		return err
	}
	return nil
}

func (network *Network) UploadFrame(buf_i int) error {
	return network.BroadcastCommand(fmt.Sprintf("dpc! upload %d;", buf_i))
}

func NetworkByInterfaceName(name string) (*Network, error) {
	const IPV6_ALL_NODES_MULTICAST_ADDRESS = "ff02::1"

	dev, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	baddr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s]:%d", IPV6_ALL_NODES_MULTICAST_ADDRESS, PROTOCOL_PORT))
	if err != nil {
		return nil, err
	}

	unicastConn, err := net.ListenUDP("udp6", nil)
	{
		if err != nil {
			return nil, err
		}
		f, err := unicastConn.File()
		if err != nil {
			return nil, err
		}

		err = syscall.BindToDevice(int(f.Fd()), dev.Name)
		if err != nil {
			return nil, err
		}
	}

	broadcastConn, err := net.ListenUDP("udp6", nil)
	{
		if err != nil {
			return nil, err
		}
		f, err := broadcastConn.File()
		if err != nil {
			return nil, err
		}

		err = syscall.BindToDevice(int(f.Fd()), dev.Name)
		if err != nil {
			return nil, err
		}
	}

	return &Network{
		UnicastConn:   unicastConn,
		BroadcastConn: broadcastConn,
		BroadcastAddr: baddr,
		Interface:     dev,
	}, nil
}

func (tile *Tile) SendFrameBuffer(buf_i int, frame []byte) error {
	// Break frame buffer into chunks and send individually
	const FRAME_CHUNK_SIZE = 1024
	buf := bytes.NewBuffer(frame)
	for offs := 0; buf.Len() > 0; offs += FRAME_CHUNK_SIZE {
		// Extract next chunk of the frame
		chunk := buf.Next(FRAME_CHUNK_SIZE)

		tile.CommandBuf.Reset()

		// Pack text command into buffer
		command := fmt.Sprintf("dpc! data %d %d;", buf_i, offs)
		if _, err := tile.CommandBuf.Write([]byte(command)); err != nil {
			return err
		}

		// Pack binary payload into buffer
		if _, err := tile.CommandBuf.Write(chunk); err != nil {
			return err
		}

		// Transmit command
		if _, err := tile.Net.UnicastConn.WriteTo(tile.CommandBuf.Bytes(), tile.Addr); err != nil {
			return err
		}
	}

	return nil
}
