# Test 1 healthcheck: 
healthcheck:
	curl localhost:4000/v1/healthcheck

#Test 2 movies:
movies:
	curl -X POST localhost:4000/v1/movies