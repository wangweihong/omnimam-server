package multihandler

import "net/http"

type RequestHandlers []func(r *http.Request) error

func NewRequestHandlers(hs ...func(r *http.Request) error) RequestHandlers {
	rh := make([]func(r *http.Request) error, 0, len(hs))
	rh = append(rh, hs...)
	return rh
}

func (m RequestHandlers) RequestHandlers(r *http.Request) error {
	for _, h := range m {
		if err := h(r); err != nil {
			return err
		}
	}
	return nil
}

type ResponseHandlers []func(r *http.Response) error

func NewResponseHandlers(hs ...func(r *http.Response) error) ResponseHandlers {
	rh := make([]func(r *http.Response) error, 0, len(hs))
	rh = append(rh, hs...)
	return rh
}

func (m ResponseHandlers) ResponseHandler(r *http.Response) error {
	for _, h := range m {
		if err := h(r); err != nil {
			return err
		}
	}
	return nil
}
