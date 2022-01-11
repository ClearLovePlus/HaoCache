package algorithm

import (
	"container/list"
	"time"

	haolog "github.com/ClearLovePlus/haorm/log"
)

//缓存的数据结构体
type Cache struct {
	//cache 的最大内存
	maxBytes int64
	//cache 已使用的内存
	nbbytes int64
	//开启缓存淘汰策略的阈值 threshold*maxBytes
	threshold float64
	//双向链表用来存储数据
	ll *list.List
	//map 哈希表用来存储key 指向数据节点
	cache map[string]*list.Element
	//是某条记录被移除时的回调函数
	OnEvicted func(key string, value Value)
}

// Value use Len to count how many bytes it takes
type Value interface {
	Len() int
}

//双向链表的各个节点的数据类型
type entry struct {
	key        string
	value      Value
	expireTime time.Time
}

//Cache 的实例化方法
func New(maxBytes int64, threshold float64, onEivcted func(key string, value Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEivcted,
		threshold: threshold,
	}
}

//cache 通过键获得对应的值的方法
func (cache *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := cache.cache[key]; ok {
		cache.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

//lru的最近最久未使用的淘汰方法
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

//主动的移除缓存键值的方法
func (c *Cache) Remove(key string) (ok bool) {
	var ele *list.Element
	if ele, ok = c.cache[key]; !ok || ele == nil {
		haolog.Error("this key is not exist")
		return true
	}
	//存储数据的链表中移除对应的数据
	c.ll.Remove(ele)
	kv := ele.Value.(*entry)
	//存储key的哈希表（map）中移除对应的key
	delete(c.cache, kv.key)
	//归还已使用的内存
	c.nbbytes -= int64(len(kv.key)) + int64(kv.value.Len())
	//回调函数不为空的话，调用回调函数
	if c.OnEvicted != nil {
		c.OnEvicted(kv.key, kv.value)
	}
	return true
}

func (c *Cache) Set(key string, value Value) (ok bool) {
	//原来的键值对已经存在则更新
	if ele, ok := c.cache[key]; ok {
		//cache 的容量不够的情况下，直接返回 先判断容量是否够
		kv := ele.Value.(*entry)
		c.nbbytes += int64(value.Len()) - int64(kv.value.Len())
		//c.maxBytes==0 表示不限制内存
		if c.maxBytes != 0 && c.nbbytes >= c.maxBytes {
			haolog.Error("the capacity of cache is not enough")
			return false
		}
		c.ll.MoveToFront(ele)
		kv.value = value
	} else {
		c.nbbytes += int64(value.Len()) + int64(len(key))
		//c.maxBytes==0 表示不限制内存
		if c.maxBytes != 0 && c.nbbytes >= c.maxBytes {
			haolog.Error("the capacity of cache is not enough")
			return false
		}
		ele := c.ll.PushFront(&entry{
			key:   key,
			value: value,
		})
		c.cache[key] = ele
	}
	//缓存淘汰的阈值
	thresholdValue := c.nbbytes * int64(c.threshold)
	for c.maxBytes != 0 && c.maxBytes < thresholdValue {
		c.RemoveOldest()
	}
	return true
}

func (c *Cache) Len() int {
	return c.ll.Len()
}
