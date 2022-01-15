gen:
	rm -rf pkg/generated
	./hack/update-codegen.sh
	cp temp/github.com/peizhong/serverless-controller/pkg/apis/serverlesscontroller/v1alpha1/zz_generated.deepcopy.go pkg/apis/serverlesscontroller/v1alpha1/
	cp -r temp/github.com/peizhong/serverless-controller/pkg/generated pkg/
	rm -rf temp
