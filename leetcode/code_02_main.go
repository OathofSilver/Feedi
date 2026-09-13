package leetcode

func findAnagrams(s string, p string) []int {
	//定长滑动窗口 + 字符计数
	n := len(s)
	m := len(p)
	if n < m {
		return nil
	}
	var parray [26]int
	var windowsarray [26]int
	res := make([]int, 0)
	// 初始化第一个窗口 和 字符串p的字符计数
	for i := 0; i < m; i++ {
		parray[p[i]-'a']++
		windowsarray[s[i]-'a']++
	}
	if parray == windowsarray {
		return []int{0}
	}
	for i := m; i < n; i++ {
		leftChar := s[i-m]
		windowsarray[leftChar-'a']--
		windowsarray[s[i]-'a']++
		if parray == windowsarray {
			res = append(res, i-m+1)
		}
	}
	return res
}

//func subarraySum(nums []int, k int) int {
//	// 前缀和法
//	// sum[i] = sum[i-1] + num[i]
//	sum := make([]int, len(nums))
//	sum[0] = nums[0]
//	for i := 1; i < len(nums); i++ {
//		sum[i] = sum[i-1] + nums[i]
//	}
//
//}

func addTwoNumbers(l1 *ListNode, l2 *ListNode) *ListNode {
	p1 := l1
	p2 := l2
	carry := 0
	head := &ListNode{} // 哑节点dummy
	q := head

	// 只要p1不为空 OR p2不为空 OR还有进位，就要继续循环
	for p1 != nil || p2 != nil || carry > 0 {
		v1, v2 := 0, 0
		if p1 != nil {
			v1 = p1.Val
			p1 = p1.Next
		}
		if p2 != nil {
			v2 = p2.Val
			p2 = p2.Next
		}
		total := v1 + v2 + carry
		digit := total % 10
		carry = total / 10
		q.Next = &ListNode{Val: digit}
		q = q.Next
	}
	return head.Next
}

func removeNthFromEnd(head *ListNode, n int) *ListNode {
	// 快慢指针+dummy 实现一趟扫描实现
	dummy := &ListNode{Next: head}
	slow, fast := dummy, dummy
	for i := 0; i <= n; i++ {
		if fast == nil { // 防御：n非法，直接返回
			break
		}
		fast = fast.Next
	}
	for fast != nil {
		slow = slow.Next
		fast = fast.Next
	}
	slow.Next = slow.Next.Next
	return dummy.Next
}

func swapPairs(head *ListNode) *ListNode {
	slow, fast := head, head.Next
	for fast != nil {
		slow.Val, fast.Val = fast.Val, slow.Val
		slow = slow.Next
		fast = fast.Next
	}
	return head
}

func setZeroes(matrix [][]int) {
	// O (m+n) 空间
	if len(matrix) == 0 {
		return
	}
	m, n := len(matrix), len(matrix[0])
	zeroRow := make([]bool, m)
	zeroCol := make([]bool, n)
	//第一轮：标记哪些行、列需要置零
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			if matrix[i][j] == 0 {
				zeroRow[i] = true
				zeroCol[j] = true
			}
		}
	}
	//第二轮：根据标记原地置零
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			if zeroRow[i] || zeroCol[j] {
				matrix[i][j] = 0
			}
		}
	}
}

//func spiralOrder(matrix [][]int) []int {
//	// 遍历第0行，删除最后一列，旋转顺时针90度
//	res := make([]int, 0)
//	for len(matrix) > 0 {
//		// 记录第0行的全部元素
//		for i := 0; i < len(matrix[0]); i++ {
//			res = append(res, matrix[0][i])
//		}
//		//删除最后一列
//
//	}
//}
