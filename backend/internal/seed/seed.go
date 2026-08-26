package seed

import (
	"context"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/domain/knowledge"
	"github.com/apa/backend/internal/domain/skill"
	"github.com/apa/backend/internal/domain/user"
)

const DefaultPassword = "password123"

type Deps struct {
	Orgs      application.OrganizationRepository
	Users     application.UserRepository
	Knowledge application.KnowledgeRepository
	Bus       *application.Bus
	Log       *slog.Logger
}

func Run(ctx context.Context, deps Deps) error {
	userCount, err := deps.Users.Count(ctx)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return nil
	}

	org, err := deps.Orgs.Create(ctx, "Acme Corp")
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(DefaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	seedUsers := []struct {
		email  string
		name   string
		role   user.Role
		skills []string
	}{
		{"sara@acme.test", "Sara (Manager)", user.RoleManager, []string{"writing", "review"}},
		{"ali@acme.test", "Ali Hassan", user.RoleMember, []string{"security", "statistics", "incident-response"}},
		{"mina@acme.test", "Mina Rahimi", user.RoleMember, []string{"writing", "communications"}},
	}

	ids := map[string]user.User{}
	for _, su := range seedUsers {
		u := &user.User{
			OrgID:  org.ID,
			Email:  su.email,
			Name:   su.name,
			Role:   su.role,
			Skills: su.skills,
		}
		if err := deps.Users.Create(ctx, u, string(hash)); err != nil {
			return err
		}
		ids[su.name] = *u
	}

	minaID := ids["Mina Rahimi"].ID
	fact, err := knowledge.NewFact(org.ID, knowledge.KindTopicOwner, "communications", minaID, 0.7, knowledge.SourceSeeded, "Seeded from HR directory: Mina leads internal communications.")
	if err != nil {
		return err
	}
	if err := deps.Knowledge.UpsertFact(ctx, fact); err != nil {
		return err
	}

	deps.Log.InfoContext(ctx, "seeded demo organization",
		slog.String("org", org.Name),
		slog.Int("users", len(seedUsers)),
		slog.String("password", DefaultPassword),
	)
	return nil
}

type SkillCatalogDeps struct {
	Orgs   application.OrganizationRepository
	Skills application.SkillRepository
	Log    *slog.Logger
}

func EnsureSkillCatalog(ctx context.Context, deps SkillCatalogDeps) error {
	org, err := deps.Orgs.First(ctx)
	if err != nil {
		return nil
	}
	count, err := deps.Skills.Count(ctx, org.ID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	catalog := []struct {
		name, desc string
		keywords   []string
	}{
		{"تحلیل داده", "جمع‌آوری، پاک‌سازی و تفسیر داده برای استخراج بینش و تهیه گزارش آماری.", []string{"داده", "آمار", "analysis", "statistics", "گزارش داده"}},
		{"برنامه‌نویسی وب", "طراحی و پیاده‌سازی وبسایت و وب‌اپلیکیشن (فرانت‌اند و بک‌اند).", []string{"وبسایت", "وب", "web", "frontend", "backend", "کدنویسی"}},
		{"طراحی رابط کاربری", "طراحی بصری صفحات، پروتوتایپ و تجربه کاربری محصول.", []string{"ui", "ux", "طراحی", "رابط", "پروتوتایپ"}},
		{"نوشتن فنی", "نگارش مستندات، گزارش‌های رسمی و محتوای ساختاریافته.", []string{"مستند", "گزارش", "نوشتن", "documentation", "report"}},
		{"امنیت سایبری", "ارزیابی و تقویت امنیت سیستم‌ها، مدیریت رخدادها و آگاهی امنیتی.", []string{"امنیت", "سایبری", "security", "incident", "نفوذ"}},
		{"آموزش", "طراحی و اجرای دوره‌ها، کارگاه‌ها و انتقال دانش به دیگران.", []string{"آموزش", "کارگاه", "دوره", "training", "teaching"}},
		{"مدیریت پروژه", "برنامه‌ریزی، زمان‌بندی و هماهنگی تیم‌ها برای تحویل پروژه.", []string{"پروژه", "برنامه‌ریزی", "project", "planning", "زمان‌بندی"}},
		{"پشتیبانی مشتری", "پاسخگویی به مشتریان، حل مشکلات و جمع‌آوری بازخورد.", []string{"مشتری", "پشتیبانی", "support", "customer", "بازخورد"}},
		{"بازاریابی", "معرفی محصول، کمپین‌های تبلیغاتی و جذب مخاطب.", []string{"بازاریابی", "تبلیغ", "marketing", "کمپین"}},
		{"منابع انسانی", "استخدام، رویboarding کارکنان و امور اداری پرسنل.", []string{"استخدام", "کارمند", "hr", "منابع انسانی", "onboarding"}},
		{"مالی و حسابداری", "ثبت اسناد مالی، بودجه‌بندی و گزارش‌های مالی.", []string{"مالی", "حسابدار", "بودجه", "finance", "accounting"}},
		{"تست نرم‌افزار", "طراحی و اجرای تست‌ها برای اطمینان از کیفیت نرم‌افزار.", []string{"تست", "کیفیت", "test", "qa", "باگ"}},
		{"تحقیق بازار", "بررسی بازار، رقبا و نیازهای مخاطبان هدف.", []string{"تحقیق", "بازار", "research", "رقبا"}},
		{"گرافیک و مالتی‌مدیا", "طراحی گرافیک، تدوین ویدیو و تولید محتوای بصری.", []string{"گرافیک", "ویدیو", "تصویر", "graphic", "video"}},
		{"حقوقی و قراردادها", "تنظیم و بازبینی قراردادها و مشاوره حقوقی سازمان.", []string{"قرارداد", "حقوق", "legal", "contract"}},
		{"اجرای رویداد", "برنامه‌ریزی و اجرای جلسات، همایش‌ها و رویدادهای داخلی.", []string{"رویداد", "همایش", "جلسه", "event"}},
	}
	for _, c := range catalog {
		sk, err := skill.New(org.ID, c.name, c.desc, c.keywords)
		if err != nil {
			return err
		}
		if err := deps.Skills.Create(ctx, sk); err != nil {
			return err
		}
	}
	deps.Log.InfoContext(ctx, "default skill catalog created", slog.Int("count", len(catalog)))
	return nil
}
