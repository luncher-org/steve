package accesscontrol

import (
	"reflect"
	"slices"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_policyRuleIndex_roleBindingBySubject(t *testing.T) {
	roleRef := rbacv1.RoleRef{Kind: "Role", Name: "testrole"}
	tests := []struct {
		name string
		kind string
		rb   *rbacv1.RoleBinding
		want []string
	}{
		{
			name: "indexes users",
			kind: "User",
			rb: makeRB("testns", "testrb", roleRef, []rbacv1.Subject{
				{
					APIGroup: rbacGroup,
					Kind:     "User",
					Name:     "myuser",
				},
			}),
			want: []string{"myuser"},
		},
		{
			name: "indexes multiple subjects",
			kind: "Group",
			rb: makeRB("testns", "testrb", roleRef, []rbacv1.Subject{
				{
					APIGroup: rbacGroup,
					Kind:     "Group",
					Name:     "mygroup1",
				},
				{
					APIGroup: rbacGroup,
					Kind:     "Group",
					Name:     "mygroup2",
				},
			}),
			want: []string{"mygroup1", "mygroup2"},
		},
		{
			name: "indexes svcaccounts in user mode",
			kind: "User",
			rb: makeRB("testns", "testrb", roleRef, []rbacv1.Subject{
				{
					APIGroup:  "",
					Kind:      "ServiceAccount",
					Name:      "mysvcaccount",
					Namespace: "testns",
				},
			}),
			want: []string{"system:serviceaccount:testns:mysvcaccount"},
		},
		{
			name: "ignores svcaccounts in group mode",
			kind: "Group",
			rb: makeRB("testns", "testrb", roleRef, []rbacv1.Subject{
				{
					APIGroup:  "",
					Kind:      "ServiceAccount",
					Name:      "mysvcaccount",
					Namespace: "testns",
				},
			}),
			want: []string{},
		},
		{
			name: "ignores unknown subjects",
			kind: "Group",
			rb: makeRB("testns", "testrb", roleRef, []rbacv1.Subject{
				{
					APIGroup: rbacGroup,
					Kind:     "User",
					Name:     "myuser",
				},
				{
					APIGroup: rbacGroup,
					Kind:     "Group",
					Name:     "mygroup1",
				},
				{
					APIGroup: "custom.api.group",
					Kind:     "CustomGroup",
					Name:     "mygroup2",
				},
			}),
			want: []string{"mygroup1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexFunc := roleBindingBySubjectIndexer(tt.kind)
			if got, err := indexFunc(tt.rb); err != nil {
				t.Error(err)
			} else if !slices.Equal(got, tt.want) {
				t.Errorf("roleBindingBySubjectIndexer() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_policyRuleIndex_clusterRoleBindingBySubject(t *testing.T) {
	roleRef := rbacv1.RoleRef{Kind: "ClusterRole", Name: "testclusterrole"}
	tests := []struct {
		name string
		kind string
		crb  *rbacv1.ClusterRoleBinding
		want []string
	}{
		{
			name: "ignores if RoleRef is a Role",
			kind: "User",
			crb: makeCRB("testcrb", rbacv1.RoleRef{Kind: "Role", Name: "testrole"}, []rbacv1.Subject{
				{
					APIGroup: rbacGroup,
					Kind:     "User",
					Name:     "myuser",
				},
			}),
			want: []string{},
		},
		{
			name: "indexes users",
			kind: "User",
			crb: makeCRB("testcrb", roleRef, []rbacv1.Subject{
				{
					APIGroup: rbacGroup,
					Kind:     "User",
					Name:     "myuser",
				},
			}),
			want: []string{"myuser"},
		},
		{
			name: "indexes multiple subjects",
			kind: "Group",
			crb: makeCRB("testcrb", roleRef, []rbacv1.Subject{
				{
					APIGroup: rbacGroup,
					Kind:     "Group",
					Name:     "mygroup1",
				},
				{
					APIGroup: rbacGroup,
					Kind:     "Group",
					Name:     "mygroup2",
				},
			}),
			want: []string{"mygroup1", "mygroup2"},
		},
		{
			name: "indexes svcaccounts in user mode",
			kind: "User",
			crb: makeCRB("testcrb", roleRef, []rbacv1.Subject{
				{
					APIGroup:  "",
					Kind:      "ServiceAccount",
					Name:      "mysvcaccount",
					Namespace: "testns",
				},
			}),
			want: []string{"system:serviceaccount:testns:mysvcaccount"},
		},
		{
			name: "ignores svcaccounts in group mode",
			kind: "Group",
			crb: makeCRB("testcrb", roleRef, []rbacv1.Subject{
				{
					APIGroup:  "",
					Kind:      "ServiceAccount",
					Name:      "mysvcaccount",
					Namespace: "testns",
				},
			}),
			want: []string{},
		},
		{
			name: "ignores unknown subjects",
			kind: "Group",
			crb: makeCRB("testcrb", roleRef, []rbacv1.Subject{
				{
					APIGroup: rbacGroup,
					Kind:     "User",
					Name:     "myuser",
				},
				{
					APIGroup: rbacGroup,
					Kind:     "Group",
					Name:     "mygroup1",
				},
				{
					APIGroup: "custom.api.group",
					Kind:     "CustomGroup",
					Name:     "mygroup2",
				},
			}),
			want: []string{"mygroup1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexFunc := clusterRoleBindingBySubjectIndexer(tt.kind)
			if got, err := indexFunc(tt.crb); err != nil {
				t.Error(err)
			} else if !slices.Equal(got, tt.want) {
				t.Errorf("clusterRoleBindingBySubjectIndexer() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func makeRB(namespace, name string, roleRef rbacv1.RoleRef, subjects []rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}
}

func makeCRB(name string, roleRef rbacv1.RoleRef, subjects []rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}
}

func Test_addResourceAccess(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		rule      rbacv1.PolicyRule
		want      AccessSet
	}{
		{
			name:      "RoleBinding namespaces resource with empty names",
			namespace: "test-ns",
			rule: rbacv1.PolicyRule{
				APIGroups:     []string{""},
				Resources:     []string{"namespaces", "deployments"},
				ResourceNames: []string{},
				Verbs:         []string{"get"},
			},
			want: AccessSet{
				set: map[key]resourceAccessSet{
					{
						verb: "get", gr: schema.GroupResource{Group: "", Resource: "namespaces"}}: {
						Access{Namespace: "*", ResourceName: "test-ns"}: true,
					},
					{
						verb: "get", gr: schema.GroupResource{Group: "", Resource: "deployments"}}: {
						Access{Namespace: "test-ns", ResourceName: "*"}: true,
					},
				},
			},
		},
		{
			name:      "ClusterRoleBinding namespaces resource with empty names",
			namespace: "*",
			rule: rbacv1.PolicyRule{
				APIGroups:     []string{""},
				Resources:     []string{"namespaces", "deployments"},
				ResourceNames: []string{},
				Verbs:         []string{"get"},
			},
			want: AccessSet{
				set: map[key]resourceAccessSet{
					{verb: "get", gr: schema.GroupResource{Group: "", Resource: "namespaces"}}: {
						Access{Namespace: "*", ResourceName: "*"}: true,
					},
					{
						verb: "get", gr: schema.GroupResource{Group: "", Resource: "deployments"}}: {
						Access{Namespace: "*", ResourceName: "*"}: true,
					},
				},
			},
		},
		{
			name:      "RoleBinding namespaces resource with specific names",
			namespace: "test-ns",
			rule: rbacv1.PolicyRule{
				APIGroups:     []string{""},
				Resources:     []string{"namespaces"},
				ResourceNames: []string{"specific-ns"},
				Verbs:         []string{"get"},
			},
			want: AccessSet{
				set: map[key]resourceAccessSet{
					{verb: "get", gr: schema.GroupResource{Group: "", Resource: "namespaces"}}: {
						Access{Namespace: "test-ns", ResourceName: "specific-ns"}: true,
					},
				},
			},
		},
		{
			name:      "RoleBinding namespaces resource with its own namespace",
			namespace: "test-ns",
			rule: rbacv1.PolicyRule{
				APIGroups:     []string{""},
				Resources:     []string{"namespaces"},
				ResourceNames: []string{"test-ns"},
				Verbs:         []string{"get"},
			},
			want: AccessSet{
				set: map[key]resourceAccessSet{
					{verb: "get", gr: schema.GroupResource{Group: "", Resource: "namespaces"}}: {
						Access{Namespace: "*", ResourceName: "test-ns"}: true,
					},
				},
			},
		},
		{
			name:      "RoleBinding other resource with empty names",
			namespace: "test-ns",
			rule: rbacv1.PolicyRule{
				APIGroups:     []string{"apps"},
				Resources:     []string{"deployments"},
				ResourceNames: []string{},
				Verbs:         []string{"get"},
			},
			want: AccessSet{
				set: map[key]resourceAccessSet{
					{verb: "get", gr: schema.GroupResource{Group: "apps", Resource: "deployments"}}: {
						Access{Namespace: "test-ns", ResourceName: "*"}: true,
					},
				},
			},
		},
		{
			name:      "ClusterRoleBinding other resource with empty names",
			namespace: "*",
			rule: rbacv1.PolicyRule{
				APIGroups:     []string{"apps"},
				Resources:     []string{"deployments"},
				ResourceNames: []string{},
				Verbs:         []string{"get"},
			},
			want: AccessSet{
				set: map[key]resourceAccessSet{
					{verb: "get", gr: schema.GroupResource{Group: "apps", Resource: "deployments"}}: {
						Access{Namespace: "*", ResourceName: "*"}: true,
					},
				},
			},
		},
		{
			name:      "RoleBinding other resource with specific names",
			namespace: "test-ns",
			rule: rbacv1.PolicyRule{
				APIGroups:     []string{"apps"},
				Resources:     []string{"deployments"},
				ResourceNames: []string{"my-deploy"},
				Verbs:         []string{"get"},
			},
			want: AccessSet{
				set: map[key]resourceAccessSet{
					{verb: "get", gr: schema.GroupResource{Group: "apps", Resource: "deployments"}}: {
						Access{Namespace: "test-ns", ResourceName: "my-deploy"}: true,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessSet := &AccessSet{}
			addResourceAccess(accessSet, tt.namespace, tt.rule)
			if !reflect.DeepEqual(*accessSet, tt.want) {
				t.Errorf("addResourceAccess() got = %v, want %v", *accessSet, tt.want)
			}
		})
	}
}
