package user_logic

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ppoonk/AirGo/constant"
	"github.com/ppoonk/AirGo/global"
	"github.com/ppoonk/AirGo/service/admin_logic"
	"github.com/ppoonk/AirGo/utils/jwt_plugin"
	timeTool "github.com/ppoonk/AirGo/utils/time_plugin"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"

	"errors"
	"github.com/ppoonk/AirGo/model"
	encrypt_plugin "github.com/ppoonk/AirGo/utils/encrypt_plugin"
)

type User struct{}

var userService *User

// 注册
func (us *User) Register(userParams *model.User) error {
	//判断是否存在
	var user model.User
	err := global.DB.Where(&model.User{UserName: userParams.UserName}).First(&user).Error
	if err == nil {
		return errors.New("User already exists")
	} else if err == gorm.ErrRecordNotFound {

		return us.CreateUser(userParams)
	} else {
		return err
	}
}

// 创建用户
func (us *User) CreateUser(u *model.User) error {
	return global.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&u).Error
	})
}

// 用户登录
func (us *User) Login(u *model.UserLoginRequest) (*model.User, error) {
	var user model.User
	err := global.DB.Where("user_name = ?", u.UserName).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.New("User does not exist")
	} else if !user.Enable {
		return nil, errors.New("User frozen")
	} else {
		if err := encrypt_plugin.BcryptDecode(u.Password, user.Password); err != nil {
			return nil, errors.New("Password error")
		}
		return &user, err
	}
}
func (us *User) GetUserToken(user *model.User) (string, error) {
	//查缓存
	cache, ok := global.LocalCache.Get(fmt.Sprintf("%s%d", constant.CACHE_USER_TOKEN_BY_ID, user.ID))
	if ok {
		return cache.(string), nil
	}
	//生成新的
	myCustomClaimsPrefix := jwt_plugin.MyCustomClaimsPrefix{
		UserID:   user.ID,
		UserName: user.UserName,
	}
	ep, _ := timeTool.ParseDuration(global.Server.Security.JWT.ExpiresTime)
	registeredClaims := jwt.RegisteredClaims{
		Issuer:    global.Server.Security.JWT.Issuer,      // 签发者
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ep)), //过期时间
		NotBefore: jwt.NewNumericDate(time.Now()),         //生效时间
	}
	tokenNew, err := jwt_plugin.GenerateTokenUsingHs256(myCustomClaimsPrefix, registeredClaims, global.Server.Security.JWT.SigningKey)
	if err != nil {
		return "", err
	}
	global.LocalCache.Set(fmt.Sprintf("%s%d", constant.CACHE_USER_TOKEN_BY_ID, user.ID), tokenNew, ep)
	return tokenNew, nil
}

// 查用户
func (us *User) FirstUser(user *model.User) (*model.User, error) {
	var userQuery model.User
	err := global.DB.Where(&user).First(&userQuery).Error
	return &userQuery, err
}

// 更新用户信息
func (us *User) UpdateUser(userParams *model.User, values map[string]any) error {
	return global.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&model.User{}).Where(&userParams).Updates(values).Error
	})
}

// 处理用户充值卡商品
func (us *User) RechargeHandle(order *model.Order) error {
	//查询商品信息
	goods, _ := shopService.FirstGoods(&model.Goods{ID: order.GoodsID})
	rechargeFloat64, _ := strconv.ParseFloat(goods.RechargeAmount, 64)
	user, err := us.FirstUser(&model.User{ID: order.UserID})
	if err != nil {
		return err
	}
	startAmount := user.Balance
	res, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", user.Balance+rechargeFloat64), 64)
	user.Balance = res
	if user.Balance < 0 {
		user.Balance = 0
	}
	err = userService.SaveUser(user)
	if err != nil {
		return err
	}
	if user.WhenBalanceChanged {
		global.GoroutinePool.Submit(func() {
			us.PushMessageWhenBalanceChanged(user, startAmount, rechargeFloat64)
		})
	}
	return nil
}

// 处理余额支付
func (us *User) UserBalancePayHandler(order *model.Order) error {
	// 查询user
	user, err := us.FirstUser(&model.User{ID: order.UserID})
	if err != nil {
		return err
	}
	startAmount := user.Balance
	totalAmount, _ := strconv.ParseFloat(order.TotalAmount, 64)
	if totalAmount == 0 {
		return nil
	}
	if user.Balance < totalAmount {
		return errors.New(constant.ERROR_BALANCE_IS_NOT_ENOUGH)
	}
	res, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", user.Balance-totalAmount), 64)
	user.Balance = res
	err = userService.SaveUser(user)
	if err != nil {
		return err
	}
	if user.WhenBalanceChanged {
		global.GoroutinePool.Submit(func() {
			us.PushMessageWhenBalanceChanged(user, startAmount, totalAmount)
		})
	}
	return nil
}
func (us *User) PushMessageWhenBalanceChanged(user *model.User, startAmount, changedAmount float64) {
	msg := admin_logic.MessageInfo{
		UserID:      user.ID,
		MessageType: admin_logic.MESSAGE_TYPE_USER,
		User:        user,
		Message: strings.Join([]string{
			"【余额变动提醒】",
			fmt.Sprintf("时间：%s", time.Now().Format("2006-01-02 15:04:05")),
			fmt.Sprintf("开始余额：%s", fmt.Sprintf("%.2f", startAmount)),
			fmt.Sprintf("结束余额：%s", fmt.Sprintf("%.2f", user.Balance)),
			fmt.Sprintf("变动值：%s\n", fmt.Sprintf("%.2f", changedAmount)),
		}, "\n"),
	}
	admin_logic.PushMessageSvc.PushMessage(&msg)
}

// 保存用户信息
func (us *User) SaveUser(u *model.User) error {
	return global.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Save(&u).Error
	})
}

// 删除cache
func (us *User) DeleteUserCacheTokenByID(user *model.User) {
	global.LocalCache.Delete(fmt.Sprintf("%s%d", constant.CACHE_USER_TOKEN_BY_ID, user.ID))
}
func (us *User) VerifyEmailWhenRegister(params model.UserRegister) (bool, error) {
	//处理邮箱验证码
	userEmail := params.UserName + params.EmailSuffix //处理邮箱后缀,注册时，用户名和邮箱后缀是分开的
	cacheEmail, ok := global.LocalCache.Get(constant.CACHE_USER_REGISTER_EMAIL_CODE_BY_USERNAME + userEmail)
	if ok {
		if !strings.EqualFold(cacheEmail.(string), params.EmailCode) {
			//验证失败，返回错误响应，但不删除缓存的验证码。因为用户输错了，需要重新输入，而不需要重新发送验证码
			return false, nil
		} else {
			//验证成功，删除缓存的验证码
			global.LocalCache.Delete(constant.CACHE_USER_REGISTER_EMAIL_CODE_BY_USERNAME + userEmail)
			return true, nil
		}
	} else {
		//cache缓存超时
		return false, errors.New("The verification code has expired, please try again")
	}

}
func (us *User) VerifyEmailWhenResetPassword(params model.UserLoginRequest) (bool, error) {
	cacheEmail, ok := global.LocalCache.Get(constant.CACHE_USER_RESET_PWD_EMAIL_CODE_BY_USERNAME + params.UserName)
	if ok {
		if !strings.EqualFold(cacheEmail.(string), params.EmailCode) {
			//验证失败，返回错误响应，但不删除缓存的验证码。因为用户输错了，需要重新输入，而不需要重新发送验证码
			return false, nil
		} else {
			//验证成功，删除缓存的验证码
			global.LocalCache.Delete(constant.CACHE_USER_RESET_PWD_EMAIL_CODE_BY_USERNAME + params.UserName)
			return true, nil
		}
	} else {
		//cache缓存超时
		return false, errors.New("The verification code has expired, please try again")
	}
}
