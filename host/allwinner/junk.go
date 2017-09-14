package allwinner

import (
	"errors"
	"fmt"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpiostream"
	"periph.io/x/periph/host/pmem"
)

/*
func smokeTestGPIO() error {
	// clockSrc defines the clock source to use for GPIO related DMA transfers.
	// - On R8, SPI1 is in the PC group, which is exposed on the NAND pins so it
	//   isn't usable anyway.
	// - On A64, the PC group is exposed on the headers but SPI2 isn't so use
	//   this clock source.
	//clockSrc ddmaR8Cfg = ddmaR8Cfg(0xFFFFFFFF)
	//clockSrc = ddmaSrcDrqSPI1RX | ddmaDstDrqSPI1TX
	//clockSrc = ddmaSrcDrqSDRAM | ddmaDstDrqSDRAM
	pDst, err := pmem.Alloc(4096)
	if err != nil {
		return err
	}
	defer pDst.Close()
	dst := pDst.Bytes()[:36]
	for i := range dst {
		dst[i] = 0x11
	}

	n := dmaMemory.getDedicated()
	if n == -1 {
		return errors.New("no channel available")
	}
	dmaMemory.irqEn &^= 3 << uint(2*n+16)
	dmaMemory.irqPendStas = 3 << uint(2*n+16)
	ch := &dmaMemory.dedicated[n]
	defer ch.release()
	ch.set(uint32(gpioBaseAddr+36+16), uint32(pDst.PhysAddr()), 4, false, false, ddmaDstDrqSDRAM|ddmaSrcDrqSDRAM)

	for ch.cfg&ddmaBusy != 0 {
	}

	src := gpioM.Bytes()[:36]
	if !bytes.Equal(src, dst) {
		return fmt.Errorf("DMA corrupted the buffer:\n%s\n%s", hex.EncodeToString(src), hex.EncodeToString(dst))
	}
	return nil
}
*/

// byteToBit packs a bit offset found on slice `d` back into a densely packed
// Bits stream.
func byteToBit(w gpiostream.Bits, d []uint8, offset uint8) {
	mask := uint8(1) << offset
	for i := range w {
		w[i] = ((d[8*i+0]&mask)>>offset<<0 |
			(d[8*i+1]&mask)>>offset<<1 |
			(d[8*i+2]&mask)>>offset<<2 |
			(d[8*i+3]&mask)>>offset<<3 |
			(d[8*i+4]&mask)>>offset<<4 |
			(d[8*i+5]&mask)>>offset<<5 |
			(d[8*i+6]&mask)>>offset<<6 |
			(d[8*i+7]&mask)>>offset<<7)
	}
}

func Stream(p *Pin, w gpiostream.Stream, period time.Duration, r gpio.Bits) error {
	// TODO(maruel): Enable half interrupt. This is useful for continuous
	// operation.
	// TODO(maruel): Have central clock management to not interfere with the
	// other on-going transfers.
	// TODO(maruel): Reuse the same physical buffer for both read and write, the
	// idea would be to start the write buffer one byte offset. This would cut
	// the memory use to 1 byte per sample, which is awesome.
	//pretty.Printf("%# v\n", dmaMemory)
	if clockMemory == nil || spiMemory == nil {
		return errors.New("subsystem not initialized")
	}
	// Reading:
	clockSrc := ddmaSrcDrqSPI1RX | ddmaDstDrqSDRAM
	// Exactly 8Mhz. It is then further slowed down via wait cycles below.
	//clockMemory.spi1Clk = spiClkDiv1a | spiClkDiv3b
	// Exactly 250kHz
	clockMemory.spi1Clk = clockSPIDiv8a | clockSPIDiv12b

	/*var dWrite *dmaDedicatedGroup
	iWrite := -1
	if w != nil {
		if iWrite = dmaMemory.getDedicated(); iWrite == -1 {
			return errors.New("allwinner-dma: could not find available DMA controller")
		}
		dWrite = &dmaMemory.dedicated[iWrite]
		defer dWrite.release()
		// Disable and clear interrupts. We are in usermode after all.
		dmaMemory.irqEn &^= 3 << uint(2*iWrite+16)
		dmaMemory.irqPendStas = 3 << uint(2*iWrite+16)
	}*/
	var dRead *dmaDedicatedGroup
	iRead := -1
	if r != nil {
		if iRead = dmaMemory.getDedicated(); iRead == -1 {
			return errors.New("allwinner-dma: could not find available DMA controller")
		}
		dRead = &dmaMemory.dedicated[iRead]
		defer dRead.release()
		// Disable and clear interrupts. We are in usermode after all.
		dmaMemory.irqEn &^= 3 << uint(2*iRead+16)
		dmaMemory.irqPendStas = 3 << uint(2*iRead+16)
	}

	// Make sure the source clock is disabled.
	clockMemory.spi1Clk &^= clockSPIEnable

	//offset := p.offset & 7
	// p.group*sizeof(gpioGroup) + sizeof(Pn_CFGx) plus offset inside Pn_DAT.
	datAddr := gpioBaseAddr + uint32(p.group)*36 + 16 + uint32(p.offset)/8
	/*
		if w != nil {
				l := int(w.Duration() / w.Resolution())
				p, err := pmem.Alloc((l + 0xFFF) &^ 0xFFF)
				if err != nil {
					return err
				}
				defer p.Close()
				if err := w.Raster8(w.Resolution(), p.Bytes(), offset); err != nil {
					return err
				}
				dWrite.set(uint32(p.PhysAddr()), datAddr, uint32(l), false, true, clockSrc)
		}
	*/
	var readBuf []byte
	if r != nil {
		l := len(r)
		p, err := pmem.Alloc((l + 0xFFF) &^ 0xFFF)
		if err != nil {
			return err
		}
		defer p.Close()
		readBuf = p.Bytes()
		dRead.set(datAddr, uint32(p.PhysAddr()), uint32(l), true, false, clockSrc)
	}

	spiMemory.groups[1].setup()
	// Exact synchronization to start both engines.
	clockMemory.spi0Clk |= clockSPIEnable

	// TODO(maruel): The goal is to not waste CPU resource while waiting for the
	// transfer.
	//time.Sleep(time.Duration(len(buf.Bytes())) * time.Second / time.Duration(speed))

	// Spin until the the bit is reset, to release the DMA controller channel.
	/*
		if dWrite != nil {
				for dWrite.cfg&ddmaBusy != 0 {
					//pretty.Printf("Write: 0x%08x\n", dWrite.cfg)
					time.Sleep(time.Second)
				}
		}
	*/
	if dRead != nil {
		// REMOVE pretty.Printf("DMA Read: %# v\n", dRead)
		/*
			fmt.Printf("IRQ En: 0x%00x\n", dmaMemory.irqEn)
			fmt.Printf("IRQ Pending: 0x%00x\n", dmaMemory.irqPendStas)
		*/
		for dRead.cfg&ddmaBusy != 0 {
			//pretty.Printf("Read: 0x%08x\n", dRead.cfg)
			time.Sleep(time.Second)
		}
		// Copy back.
		// TODO(maruel): Temporary hack.
		copy(r, readBuf)
		//byteToBit(r, readBuf, offset)
		// TODO(maruel): Temporary.
		//fmt.Printf("%s\n", hex.EncodeToString(readBuf))
		//fmt.Printf("%x\n", gpioMemory.groups[p.group].data)
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// spi2ReadDMA initiates a read on SPI2_MISO via DMA.
func spi2ReadDMA(r []byte) error {
	if clockMemory == nil || dmaMemory == nil || spiMemory == nil {
		return errors.New("subsystem not initialized")
	}
	iRead := dmaMemory.getDedicated()
	if iRead == -1 {
		return errors.New("allwinner-dma: could not find available DMA controller")
	}
	dRead := &dmaMemory.dedicated[iRead]
	defer dRead.release()
	pDst, err := pmem.Alloc((len(r) + 0xFFF) &^ 0xFFF)
	if err != nil {
		return err
	}
	// Make sure the source clock is disabled. Set it at 250kHz.
	clockMemory.spi2Clk &^= clockSPIEnable
	clockMemory.spi2Clk = clockSPIDiv8a | clockSPIDiv12b
	// Disable and clear interrupts. We are in usermode after all.
	dmaMemory.irqEn &^= 3 << uint(2*iRead+16)
	dmaMemory.irqPendStas = 3 << uint(2*iRead+16)
	// Read SPI2RX, write to DRAM.
	fmt.Printf("setup\n")
	spiMemory.groups[2].setup()
	dRead.set(spiBaseAddr+0x2000+0x300, uint32(pDst.PhysAddr()), uint32(len(r)), true, false, ddmaSrcDrqSPI2RX|ddmaDstDrqSDRAM)

	// Start.
	clockMemory.spi2Clk |= clockSPIEnable
	for i := 0; dRead.cfg&ddmaBusy != 0 && i < 10; i++ {
		// REMOVE pretty.Printf("Read: 0x%08x\n", dRead.cfg)
		time.Sleep(time.Second)
	}
	copy(r, pDst.Bytes())
	fmt.Printf("Done\n")
	return nil
}

func spiTransmit(w, r []byte) error {
	// TODO(maruel): Have central clock management to not interfere with the
	// other on-going transfers.
	// TODO(maruel): Reuse the same physical buffer for both read and write, the
	// idea would be to start the write buffer one byte offset. This would cut
	// the memory use to 1 byte per sample, which is awesome.
	if clockMemory == nil || spiMemory == nil {
		return errors.New("subsystem not initialized")
	}
	// Exactly 8Mhz. It is then further slowed down via wait cycles below.
	//clockMemory.spi1Clk = spiClkDiv1a | spiClkDiv3b
	// Exactly 250kHz
	clockMemory.spi1Clk = clockSPIDiv8a | clockSPIDiv12b

	/*var dWrite *dmaDedicatedGroup
	iWrite := -1
	if w != nil {
		if iWrite = dmaMemory.getDedicated(); iWrite == -1 {
			return errors.New("allwinner-dma: could not find available DMA controller")
		}
		dWrite = &dmaMemory.dedicated[iWrite]
		defer dWrite.release()
		// Disable and clear interrupts. We are in usermode after all.
		dmaMemory.irqEn &^= 3 << uint(2*iWrite+16)
		dmaMemory.irqPendStas = 3 << uint(2*iWrite+16)
	}*/
	var dRead *dmaDedicatedGroup
	iRead := -1
	if r != nil {
		if iRead = dmaMemory.getDedicated(); iRead == -1 {
			return errors.New("allwinner-dma: could not find available DMA controller")
		}
		dRead = &dmaMemory.dedicated[iRead]
		defer dRead.release()
		// Disable and clear interrupts. We are in usermode after all.
		dmaMemory.irqEn &^= 3 << uint(2*iRead+16)
		dmaMemory.irqPendStas = 3 << uint(2*iRead+16)
	}

	// Make sure the source clock is disabled.
	clockMemory.spi1Clk &^= clockSPIEnable

	//offset := p.offset & 7
	// p.group*sizeof(gpioGroup) + sizeof(Pn_CFGx) plus offset inside Pn_DAT.
	//datAddr := gpioBaseAddr + uint32(p.group)*36 + 16 + uint32(p.offset)/8
	/*
		if w != nil {
				l := int(w.Duration() / w.Resolution())
				p, err := pmem.Alloc((l + 0xFFF) &^ 0xFFF)
				if err != nil {
					return err
				}
				defer p.Close()
				if err := w.Raster8(w.Resolution(), p.Bytes(), offset); err != nil {
					return err
				}
				dWrite.set(uint32(p.PhysAddr()), datAddr, uint32(l), false, true, clockSrc)
		}
	*/
	//var readBuf []byte
	if r != nil {
		/*
			l := len(r.Bits)
			p, err := pmem.Alloc((l + 0xFFF) &^ 0xFFF)
			if err != nil {
				return err
			}
			defer p.Close()
			readBuf = p.Bytes()
			dRead.set(datAddr, uint32(p.PhysAddr()), uint32(l), true, false, clockSrc)
		*/
	}

	spiMemory.groups[1].setup()
	// Exact synchronization to start both engines.
	clockMemory.spi0Clk |= clockSPIEnable

	// TODO(maruel): The goal is to not waste CPU resource while waiting for the
	// transfer.
	//time.Sleep(time.Duration(len(buf.Bytes())) * time.Second / time.Duration(speed))

	// Spin until the the bit is reset, to release the DMA controller channel.
	/*
		if dWrite != nil {
				for dWrite.cfg&ddmaBusy != 0 {
					//pretty.Printf("Write: 0x%08x\n", dWrite.cfg)
					time.Sleep(time.Second)
				}
		}
	*/
	if dRead != nil {
		// REMOVE pretty.Printf("DMA Read: %# v\n", dRead)
		/*
			fmt.Printf("IRQ En: 0x%00x\n", dmaMemory.irqEn)
			fmt.Printf("IRQ Pending: 0x%00x\n", dmaMemory.irqPendStas)
		*/
		for dRead.cfg&ddmaBusy != 0 {
			//pretty.Printf("Read: 0x%08x\n", dRead.cfg)
			time.Sleep(time.Second)
		}
		// Copy back.
		// TODO(maruel): Temporary hack.
		//copy(r.Bits, readBuf)
		//byteToBit(r.Bits, readBuf, offset)
		// TODO(maruel): Temporary.
		//fmt.Printf("%s\n", hex.EncodeToString(readBuf))
		//fmt.Printf("%x\n", gpioMemory.groups[p.group].data)
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}
