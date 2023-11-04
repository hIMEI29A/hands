package models

type IdMaker interface {
	MakeID() string
}
