package binpack

import (
	"context"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

// Bin-packing problem: https://en.wikipedia.org/wiki/Bin_packing_problem

// ErrItemTooLarge signals that an item was larger than the bin size so will never fit any bin.
var ErrItemTooLarge = errors.New("item too large for bin")

// Bin is a container into which Items are packed.
type Bin interface {
	GetID() uint64
}

// Item is implemented by types which are packed into Bins.
type Item interface{}

// Space is the size unit.
type Space uint

// GetID returns 0.
func (Space) GetID() uint64 {
	return 0
}

// NaivePacker implements a single-bin packing strategy.
type NaivePacker struct {
	bin    Bin
	binner Binner
}

var _ Packer = &NaivePacker{}

// Future work to include implementing FirstFitPacker, then ModifiedFirstFitPacker, as needed.

// Packer is implemented by types defining a packing strategy.
type Packer interface {
	InitWithNewBin(Binner) (Bin, error)
	InitWithCurrentBin(Binner)
	AddItem(context.Context, Item) (AddItemResult, error)
}

// Binner is implemented by types which handle concrete binning of items.
type Binner interface {
	AddItem(context.Context, Item, Bin) error
	BinSize() Space
	CloseBin(Bin)
	ItemSize(Item) Space
	NewBin() (Bin, error)
	SpaceAvailable(bin Bin) Space
	GetCurrentBin() Bin
}

// InitWithNewBin implements Packer, associating it with a concrete Binner.
func (np *NaivePacker) InitWithNewBin(binner Binner) (Bin, error) {
	np.binner = binner
	bin, err := binner.NewBin()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new bin")
	}
	np.bin = bin
	return bin, nil
}

// InitWithCurrentBin implements Packer, associating it with a concrete Binner which has been previously initialized.
func (np *NaivePacker) InitWithCurrentBin(binner Binner) {
	np.binner = binner
	np.bin = binner.GetCurrentBin()
}

// NewNaivePacker allocates and initializes a NaivePacker and an initial Binner, returning them along with any error.
func NewNaivePacker(binner Binner) (Packer, Bin, error) {
	packer := &NaivePacker{}
	bin, err := packer.InitWithNewBin(binner)
	return packer, bin, errors.Wrap(err, "failed to initialize packer")
}

// AddItemResult represents the result of an AddItem method call.
type AddItemResult struct {
	AddedToBin Bin
	NextBin    Bin
}

// AddItem takes a context and an item, and adds the item according to the naive packing strategy.
// Returns the bin to which the item was added and the bin to which the next item should be added,
// and any error.
func (np *NaivePacker) AddItem(ctx context.Context, item Item) (AddItemResult, error) {
	binner := np.binner
	bin := np.bin
	size := binner.ItemSize(item)

	if size > binner.BinSize() {
		return AddItemResult{}, ErrItemTooLarge
	}

	var result AddItemResult
	if size > binner.SpaceAvailable(bin) {
		newBin, err := np.closeBinAndOpenNew(ctx, bin)
		if err != nil {
			return AddItemResult{}, err
		}

		if err := np.addItemToBin(ctx, item, newBin); err != nil {
			return AddItemResult{}, err
		}

		result = AddItemResult{
			AddedToBin: newBin,
			NextBin:    newBin,
		}
	} else if size == binner.SpaceAvailable(bin) {
		if err := np.addItemToBin(ctx, item, bin); err != nil {
			return AddItemResult{}, err
		}

		newBin, err := np.closeBinAndOpenNew(ctx, bin)
		if err != nil {
			return AddItemResult{}, err
		}

		result = AddItemResult{
			AddedToBin: bin,
			NextBin:    newBin,
		}
	} else {
		if err := np.addItemToBin(ctx, item, bin); err != nil {
			return AddItemResult{}, err
		}

		result = AddItemResult{
			AddedToBin: bin,
			NextBin:    bin,
		}
	}

	return result, nil
}

func (np *NaivePacker) addItemToBin(ctx context.Context, item Item, bin Bin) error {
	if err := np.binner.AddItem(ctx, item, bin); err != nil {
		return errors.Wrap(err, "failed to add item to bin")
	}

	return nil
}

func (np *NaivePacker) closeBinAndOpenNew(ctx context.Context, bin Bin) (Bin, error) {
	np.binner.CloseBin(bin)

	newBin, err := np.binner.NewBin()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new bin")
	}

	return newBin, nil
}
