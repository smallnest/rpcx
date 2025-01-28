package server

// UpdateHandler 批量更新router
// 服务器使用plugin热更时，批量替换特定接口
func (s *Server) UpdateHandler(router map[string]Handler) {
	newRouter := make(map[string]Handler)
	for k, v := range s.router {
		newRouter[k] = v
	}
	for k, v := range router {
		newRouter[k] = v
	}
	s.router = newRouter
}
