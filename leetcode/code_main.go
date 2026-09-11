package leetcode

import "sort"

type ListNode struct {
	Val  int
	Next *ListNode
}

func hasCycle(head *ListNode) bool {
	// 1 哈希法
	if head == nil || head.Next == nil {
		return false
	}
	map1 := make(map[*ListNode]bool)
	for head != nil {
		if map1[head] {
			return true
		}
		map1[head] = true
		head = head.Next
	}
	return false
}

func detectCycle(head *ListNode) *ListNode {
	// 找到环的入口
	if head == nil || head.Next == nil {
		return nil
	}
	// 快慢指针法
	slow := head
	fast := head
	iscycle := false
	for fast != nil && fast.Next != nil {
		slow = slow.Next
		fast = fast.Next.Next
		if slow == fast {
			iscycle = true
			break
		}
	}
	// 如果有环
	// slow 指针重新回到head 节点，快慢指针同步前进，相遇点即为环的入口
	if iscycle {
		slow = head
		for slow != fast {
			slow = slow.Next
			fast = fast.Next
		}
		return slow
	} else {
		return nil
	}
}

func twoSum(nums []int, target int) []int {
	// 哈希法
	map1 := make(map[int]int)
	for i := 0; i < len(nums); i++ {
		if map1[target-nums[i]] != 0 {
			return []int{map1[target-nums[i]], i}
		}
		map1[nums[i]] = i
	}
	return nil
}

func groupAnagrams(strs []string) [][]string {
	// 哈希法
	// 字符串内部字符排序，异位词排序后字符串完全一致，以此作为 map 的键
	map1 := make(map[string][]string)
	for i := 0; i < len(strs); i++ {
		runes := []rune(strs[i])
		sort.Slice(runes, func(i, j int) bool {
			return runes[i] < runes[j]
		})
		map1[string(runes)] = append(map1[string(runes)], strs[i])
	}
	res := make([][]string, 0, len(map1))
	for _, v := range map1 {
		res = append(res, v)
	}
	return res

}

func longestConsecutive(nums []int) int {
	// 哈希法
	map1 := make(map[int]bool)
	// 去重
	for _, v := range nums {
		map1[v] = true
	}
	// 判断是否为启动， 启动即为当前数字的前一个数字不存在于 map 中
	// 不断的+1
	res := 0
	for k := range map1 {
		if !map1[k-1] { // 当前一个不存在
			temp := 1
			cur := k + 1
			for map1[cur] {
				temp++
				cur++
			}
			if temp > res {
				res = temp
			}
		}
	}
	return res
}

func moveZeroes(nums []int) {
	// 双指针法 只移动非零元素
	slow := 0
	for i := 0; i < len(nums); i++ {
		if nums[i] != 0 {
			nums[slow], nums[i] = nums[i], nums[slow]
			slow++
		}
	}

	for i := slow; i < len(nums); i++ {
		nums[i] = 0
	}
}

func maxArea(height []int) int {
	// 双指针法
	left, right := 0, len(height)-1
	res := 0
	for left < right {
		h := min(height[left], height[right])
		w := right - left
		res = max(res, h*w)
		if height[left] < height[right] {
			left++
		} else {
			right--
		}
	}
	return res
}

func threeSum(nums []int) [][]int {
	// a + b +	c = 0
	// c = -(a+b)  先要排序
	sort.Ints(nums)
	res := make([][]int, 0)
	n := len(nums)
	for i := 0; i < n-2; i++ {
		// 去重c
		if i > 0 && nums[i] == nums[i-1] {
			continue
		}
		target := -nums[i]
		left, right := i+1, n-1
		for left < right {
			sum := nums[left] + nums[right]
			if sum < target {
				left++
			} else if sum > target {
				right--
			} else {
				res = append(res, []int{nums[i], nums[left], nums[right]})
				// 去重a
				for left < right && nums[left] == nums[left+1] {
					left++
				}
				// 去重b
				for left < right && nums[right] == nums[right-1] {
					right--
				}
				left++
				right--
			}
		}
	}
	return res
}

func getIntersectionNode(headA, headB *ListNode) *ListNode {
	// 哈希法
	if headA == nil || headB == nil {
		return nil
	}
	map1 := make(map[*ListNode]bool)
	node := headA
	for node != nil {
		map1[node] = true
		node = node.Next
	}
	node = headB
	for node != nil {
		if map1[node] {
			return node
		}
		node = node.Next
	}
	return nil
}

func getIntersectionNode(headA, headB *ListNode) *ListNode {
    // 双指针法
	// 如果相交 a + c + b = b + c + a 路程是一样的 最后是相交点
	// 不相交， a + b = b + a 最后 都是nil
	if headA == nil || headB == nil {
		return nil
	}

	pa, pb := headA, headB
	for pa != pb {
		if pa == nil {
			pa = headB
		} else {
			pa = pa.Next
		}
		if pb == nil {
			pb = headA
		} else {
			pb = pb.Next
		}
	}
	return pa
}

// func reverseList(head *ListNode) *ListNode {
// 	// 双指针 
// 	// `pre` 前一个节点，`cur` 当前节点，`nextTemp`保存下一个节点
// 	var pre *ListNode
//     cur := head
// 	for cur != nil  {
// 		nextTemp := cur.Next
// 		cur.Next = pre
// 		pre = cur
// 		cur = nextTemp
// 	}
// 	return pre
// }

// func reverseList(head *ListNode) *ListNode {
// 	// 递归法
// 	if head == nil || head.Next == nil{
// 		return head
// 	}
// 	newHead := reverseList(head.Next)
//     head.Next.Next = head
//     head.Next = nil
//     return newHead
// }

func isPalindrome(head *ListNode) bool {
    // 数组辅助 
	// 数组 双指针
	p := head 
	array := make([]int,0)
	for p != nil {
		array = append(array, p.Val)
	}
	left, right := 0, len(array) - 1
	for left < right {
		if array[left] != array[right] {
			return false
		}
	}
	return true
}


func isPalindrome(head *ListNode) bool {
	// 快慢指针 + 反转后半部分
	if head == nil || head.Next  == nil {
		return true
	}
	// 1.快慢指针找中点
	slow,fast := head,head
	for fast.Next != nil && fast.Next.Next != nil {
		slow = slow.Next
		fast = fast.Next.Next
	}
	// 反转后半部分
	reversehead := reverseList(slow.Next)
	p1, p2 := head, reversehead
	for p1 != nil && p2 != nil {
		if p1.Val != p2.Val {
			return false
		}
		p1 = p1.Next
		p2 = p2.Next
	}
	return true
}

func reverseList(node *ListNode) *ListNode {
	var pre *ListNode
	cur := node 
	for cur != nil {
		nextnode := cur.Next
		cur.Next = pre
		pre = cur
		cur = nextnode
	}
	return pre
}




func mergeTwoLists(list1 *ListNode, list2 *ListNode) *ListNode {
	// 数组辅助法 o(n)
	array := make([]int, 0)
	p := list1
	for p != nil {
		array = append(array, p.Val)
		p = p.Next // 移动指针
	}
	p = list2
	for p != nil {
		array = append(array, p.Val)
		p = p.Next // 移动指针
	}
	sort.Ints(array)

	dummy := &ListNode{}
	cur := dummy
	for _, v := range array {
		node := &ListNode{Val: v}
		cur.Next = node
		cur = node
	}
	return dummy.Next // 返回真正头节点
}

func mergeTwoLists(list1 *ListNode, list2 *ListNode) *ListNode {
	// 递归法
	if list1 == nil {
		return list2
	}
	if list2 == nil {
		return list1
	}
	if list1.Val <= list2.Val {
		list1.Next = mergeTwoLists(list1.Next, list2)
		return list1
	} else {
		list2.Next = mergeTwoLists(list1, list2.Next)
		return list2
	}
}

func lengthOfLongestSubstring(s string) int {
	// 双指针 + 哈希表
	left, right := 0, 0
	window := make(map[byte]bool)
	res := 0

	for right < len(s) {
		c := s[right]

		// 如果当前字符已经在窗口里，就不断移动左指针，直到重复字符被移出
		for window[c] {
			delete(window, s[left])
			left++
		}

		window[c] = true

		if right-left+1 > res {
			res = right-left+1
		}

		right++
	}

	return res
}