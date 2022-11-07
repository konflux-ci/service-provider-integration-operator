package metrics

//
//type mockTransactionGatherer struct {
//	g             prometheus.Gatherer
//	gatherInvoked int
//	doneInvoked   int
//}
//
//func (g *mockTransactionGatherer) Gather() (_ []*dto.MetricFamily, done func(), err error) {
//	g.gatherInvoked++
//	mfs, err := g.g.Gather()
//	return mfs, func() { g.doneInvoked++ }, err
//}
