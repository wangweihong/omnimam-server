package example

// deepcopy-gen --input-dirs=./tools/deepcopy-gen/example --output-base=../.
type Inner interface {
	Function() float64
	DeepCopyInner() Inner
}

// +k8s:deepcopy-gen=true
type Ttest struct {
	// build-in
	Byte    byte
	Int16   int16
	Int32   int32
	Int64   int64
	Uint8   uint8
	Uint16  uint16
	Uint32  uint32
	Uint64  uint64
	Float32 float32
	Float64 float64
	String  string

	// interface
	I []Inner

	// maps
	MapByte         map[string]byte
	MapInt16        map[string]int16
	MapInt32        map[string]int32
	MapInt64        map[string]int64
	MapUint8        map[string]uint8
	MapUint16       map[string]uint16
	MapUint32       map[string]uint32
	MapUint64       map[string]uint64
	MapFloat32      map[string]float32
	MapFloat64      map[string]float64
	MapString       map[string]string
	MapStringPtr    map[string]*string
	MapStringPtrPtr map[string]**string
	MapMap          map[string]map[string]string
	MapMapPtr       map[string]*map[string]string
	MapSlice        map[string][]string
	MapSlicePtr     map[string]*[]string
	MapStruct       map[string]Ttest
	MapStructPtr    map[string]*Ttest

	// pointer
	PointerBuiltin   *string
	PointerPtr       **string
	PointerMap       *map[string]string
	PointerSlice     *[]string
	PointerMapPtr    **map[string]string
	PointerSlicePtr  **[]string
	PointerStruct    *Ttest
	PointerStructPtr **Ttest

	// slice
	SliceByte         []byte
	SliceInt16        []int16
	SliceInt32        []int32
	SliceInt64        []int64
	SliceUint8        []uint8
	SliceUint16       []uint16
	SliceUint32       []uint32
	SliceUint64       []uint64
	SliceFloat32      []float32
	SliceFloat64      []float64
	SliceString       []string
	SliceStringPtr    []*string
	SliceStringPtrPtr []**string
	SliceMap          []map[string]string
	SliceMapPtr       []*map[string]string
	SliceSlice        [][]string
	SliceSlicePtr     []*[]string
	SliceStruct       []Ttest
	SliceStructPtr    []*Ttest
}

type Embedded struct {
	EmbeddedName string
}
