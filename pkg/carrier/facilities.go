package carrier

func StringFrom(from UTF8String) String {
	return (*new(String)).FromUTF8String(from)
}

func JSONFrom(from UTF8String) JSON {
	return (*new(JSON)).FromUTF8String(from)
}

func ParcelFrom(from UTF8String) Parcel {
	return (*new(Parcel)).FromUTF8String(from)
}
